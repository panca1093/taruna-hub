"use client";

import { use, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { AlertDialog, Button } from "@heroui/react";
import { AlertTriangle, ChevronLeft, ChevronRight, CloudOff, Maximize, Send } from "lucide-react";
import { CountdownRing } from "@/components/countdown-ring";
import { LatexText } from "@/components/latex";
import { QuestionTypeBadge } from "@/components/status-badge";
import { LanguageToggle } from "@/components/language-toggle";
import { api, ApiError, type RuntimeSubtest } from "@/lib/api/client";
import { useCommonT } from "@/lib/i18n/dict/common";
import { useTryoutSubtestT } from "@/lib/i18n/dict/tryout-subtest";
import { useAnswerSync, useServerClock, useViolationReporter, type InitialAnswer } from "@/lib/tryout-session";
import { cn } from "@/lib/cn";

const RESYNC_MS = 120_000;
const FLUSH_WAIT_MS = 4_000;

interface Loaded {
  title: string;
  sessionId: string;
  data: RuntimeSubtest;
  initial: InitialAnswer[];
}

export default function SubtestPage({ params }: { params: Promise<{ id: string; subtestId: string }> }) {
  const { id, subtestId } = use(params);
  const router = useRouter();
  const t = useTryoutSubtestT();
  const commonT = useCommonT();

  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [failed, setFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [index, setIndex] = useState(0);
  const [remaining, setRemaining] = useState<number | null>(null);
  const [fullscreenWarning, setFullscreenWarning] = useState(false);
  const [screenshotWarning, setScreenshotWarning] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [portalContainer, setPortalContainer] = useState<HTMLDivElement | null>(null);
  const [deadline, setDeadline] = useState<number | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const submittedRef = useRef(false);
  const expiringRef = useRef(false);
  const clock = useServerClock();

  const leave = useCallback(
    (completed: boolean) => {
      if (document.fullscreenElement) document.exitFullscreen?.().catch(() => {});
      router.replace(completed ? `/student/tryout/${id}/result` : `/student/tryout/${id}`);
    },
    [id, router],
  );

  // The server closed this subtest (deadline, or submitted elsewhere): the overview shows where things stand.
  const handleClosed = useCallback(() => {
    if (submittedRef.current) return;
    submittedRef.current = true;
    leave(false);
  }, [leave]);

  useEffect(() => {
    let cancelled = false;
    api.runtime
      .overview(id)
      .then(async (overview) => {
        if (!overview.current) throw new ApiError(409, { code: "subtest_not_running", message: "" });
        const sessionId = overview.current.id;
        const data = await api.runtime.subtest(sessionId, subtestId);
        if (cancelled) return;
        clock.sync(data.server_time);
        setDeadline(data.subtest.deadline_at ? new Date(data.subtest.deadline_at).getTime() : null);
        setLoaded({
          title: overview.title,
          sessionId,
          data,
          initial: data.questions.map((q) => ({ id: q.id, answer: q.answer, seq: q.seq })),
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status >= 400 && err.status < 500) {
          submittedRef.current = true;
          leave(false);
          return;
        }
        setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [id, subtestId, clock, leave, reloadKey]);

  const sessionId = loaded?.sessionId ?? null;
  const questions = useMemo(() => loaded?.data.questions ?? [], [loaded]);
  const loadAnswer = useCallback(
    async (questionId: string) => {
      if (!sessionId) return null;
      const data = await api.runtime.subtest(sessionId, subtestId);
      const q = data.questions.find((item) => item.id === questionId);
      return q ? { id: q.id, answer: q.answer, seq: q.seq } : null;
    },
    [sessionId, subtestId],
  );
  const reportViolation = useViolationReporter(sessionId);
  const { answers, states, setAnswer, pendingCount, waitIdle } = useAnswerSync(
    sessionId,
    loaded?.initial ?? null,
    handleClosed,
    loadAnswer,
  );

  const doSubmit = useCallback(async () => {
    if (!sessionId || submittedRef.current) return;
    submittedRef.current = true;
    try {
      const session = await api.runtime.submitSubtest(sessionId, subtestId);
      leave(session.status === "completed");
    } catch (err) {
      if (err instanceof ApiError && err.status >= 400 && err.status < 500) {
        leave(false);
        return;
      }
      submittedRef.current = false;
      setFailed(true);
    }
  }, [sessionId, subtestId, leave]);

  // At zero the server decides: it settles on its own at the cutoff, so flush what is pending, then read the result.
  const settleExpired = useCallback(async () => {
    if (!sessionId || expiringRef.current || submittedRef.current) return;
    expiringRef.current = true;
    try {
      await waitIdle(FLUSH_WAIT_MS);
      const session = await api.runtime.session(sessionId);
      clock.sync(session.server_time);
      const part = session.subtests.find((p) => p.subtest_id === subtestId);
      if (!part || part.status !== "running") {
        submittedRef.current = true;
        leave(session.status === "completed");
        return;
      }
      if (part.deadline_at) setDeadline(new Date(part.deadline_at).getTime());
    } catch (err) {
      if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 401 && err.status !== 429) {
        submittedRef.current = true;
        leave(false);
      }
    } finally {
      expiringRef.current = false;
    }
  }, [sessionId, subtestId, clock, leave, waitIdle]);

  // Countdown: server time at the last sync plus monotonic elapsed time. It runs on server deadlines only.
  useEffect(() => {
    if (deadline === null) return;
    const tick = () => {
      const now = clock.now();
      if (now === null) return;
      const left = Math.max(0, Math.ceil((deadline - now) / 1000));
      setRemaining(left);
      if (left <= 0) void settleExpired();
    };
    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, [deadline, clock, settleExpired]);

  // Re-sync with the server clock and deadline on focus, and now and then as a keep-alive for long subtests.
  useEffect(() => {
    if (!sessionId) return;
    const resync = () => {
      api.runtime
        .session(sessionId)
        .then((session) => {
          clock.sync(session.server_time);
          const part = session.subtests.find((p) => p.subtest_id === subtestId);
          if (!part || part.status !== "running") {
            handleClosed();
            return;
          }
          if (part.deadline_at) setDeadline(new Date(part.deadline_at).getTime());
        })
        .catch(() => {});
    };
    const onVisible = () => {
      if (document.visibilityState === "visible") resync();
    };
    window.addEventListener("focus", resync);
    document.addEventListener("visibilitychange", onVisible);
    const interval = setInterval(resync, RESYNC_MS);
    return () => {
      window.removeEventListener("focus", resync);
      document.removeEventListener("visibilitychange", onVisible);
      clearInterval(interval);
    };
  }, [sessionId, subtestId, clock, handleClosed]);

  // Fullscreen is best-effort: a device without the API (e.g. iPhone Safari) takes the tryout with no enforcement and
  // no violation. Leaving fullscreen after having entered it is recorded; the timer keeps running either way.
  useEffect(() => {
    if (!loaded || !containerRef.current?.requestFullscreen) return;
    let entered = false;
    containerRef.current.requestFullscreen().catch(() => {
      // Refused without a user gesture: offer the button, but nothing was left yet, so nothing is recorded.
      if (!submittedRef.current) setFullscreenWarning(true);
    });
    const handleChange = () => {
      if (document.fullscreenElement) {
        entered = true;
        setFullscreenWarning(false);
        return;
      }
      if (!entered || submittedRef.current) return;
      setFullscreenWarning(true);
      reportViolation("exit_fullscreen");
    };
    document.addEventListener("fullscreenchange", handleChange);
    return () => document.removeEventListener("fullscreenchange", handleChange);
  }, [loaded, reportViolation]);

  // Screenshot shortcuts: best-effort deterrent only — OS-level capture can't be reliably blocked from the browser.
  useEffect(() => {
    if (!loaded) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      const isPrintScreen = e.key === "PrintScreen";
      const isMacCapture = e.metaKey && e.shiftKey && ["3", "4", "5"].includes(e.key);
      const isWinSnip = e.metaKey && e.shiftKey && e.key.toLowerCase() === "s";
      if (!isPrintScreen && !isMacCapture && !isWinSnip) return;
      e.preventDefault();
      setScreenshotWarning(true);
      reportViolation("screenshot_attempt");
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [loaded, reportViolation]);

  if (failed) {
    return (
      <div className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-3 bg-background p-6 text-center">
        <CloudOff className="size-8 text-muted" />
        <p className="text-sm font-medium text-foreground">{t.loadErrorTitle}</p>
        <p className="text-xs text-muted">{t.loadErrorDescription}</p>
        <Button
          variant="primary"
          size="sm"
          onPress={() => {
            setFailed(false);
            setReloadKey((key) => key + 1);
          }}
        >
          {t.retry}
        </Button>
      </div>
    );
  }
  if (!loaded || questions.length === 0) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-background">
        <p className="text-sm text-muted">{t.loading}</p>
      </div>
    );
  }

  const subtest = loaded.data.subtest;
  const question = questions[Math.min(index, questions.length - 1)];
  const answered = questions.filter((q) => (answers[q.id]?.length ?? 0) > 0);
  const given = answers[question.id] ?? [];

  const toggleMulti = (optionId: string) => {
    setAnswer(question.id, given.includes(optionId) ? given.filter((o) => o !== optionId) : [...given, optionId]);
  };

  const minutes = remaining !== null ? Math.floor(remaining / 60) : 0;
  const seconds = remaining !== null ? remaining % 60 : 0;
  const urgent = remaining !== null && remaining <= 60;
  const percentLeft = remaining !== null ? Math.min(100, (remaining / subtest.duration_seconds) * 100) : 100;

  return (
    <div
      ref={(node) => {
        containerRef.current = node;
        setPortalContainer(node);
      }}
      className="fixed inset-0 z-50 flex flex-col bg-background"
    >
      <header className="flex h-16 shrink-0 items-center gap-4 border-b border-border px-5">
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-foreground">{subtest.name}</p>
          <p className="truncate text-xs text-muted">{loaded.title}</p>
        </div>
        <div className="flex items-center gap-3">
          {pendingCount > 0 && <span className="text-xs text-warning">{t.pendingCount(pendingCount)}</span>}
          <CountdownRing value={percentLeft} size={40} strokeWidth={4} urgent={urgent} animate={false}>
            <span className={cn("font-mono text-[11px] font-semibold", urgent ? "text-danger" : "text-foreground")}>
              {String(minutes).padStart(2, "0")}:{String(seconds).padStart(2, "0")}
            </span>
          </CountdownRing>
          <LanguageToggle />
          <Button variant="primary" size="sm" onPress={() => setConfirmOpen(true)}>
            <Send className="size-3.5" /> {t.submitButton}
          </Button>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <main className="flex flex-1 flex-col overflow-y-auto">
          <div className="mx-auto w-full max-w-2xl flex-1 px-5 py-8">
            <div className="mb-4 flex items-center justify-between">
              <span className="text-xs font-medium text-muted">{t.questionProgress(index + 1, questions.length)}</span>
              <span className="flex items-center gap-2">
                <span className={cn("text-xs", states[question.id] === "pending" ? "text-warning" : "text-muted")}>
                  {states[question.id] === "pending" ? t.answerPending : t.answerSaved}
                </span>
                <QuestionTypeBadge type={question.type} />
              </span>
            </div>

            <div className="text-[15px] leading-relaxed text-foreground">
              <LatexText text={question.content} />
            </div>

            <div className="mt-6 space-y-2.5">
              {(question.type === "pg" || question.type === "benar_salah") &&
                question.options.map((opt) => {
                  const selected = given[0] === opt.id;
                  return (
                    <button
                      key={opt.id}
                      onClick={() => setAnswer(question.id, [opt.id])}
                      className={cn(
                        "flex w-full items-center gap-3 rounded-lg border px-4 py-3 text-left text-sm transition-colors",
                        selected
                          ? "border-accent bg-accent-soft text-accent-soft-foreground"
                          : "border-border text-foreground hover:border-accent/40 hover:bg-surface-secondary",
                      )}
                    >
                      <span
                        className={cn(
                          "flex size-4 shrink-0 items-center justify-center rounded-full border-2",
                          selected ? "border-accent" : "border-border",
                        )}
                      >
                        {selected && <span className="size-2 rounded-full bg-accent" />}
                      </span>
                      <LatexText text={opt.label} />
                    </button>
                  );
                })}

              {question.type === "pg_multi" &&
                question.options.map((opt) => {
                  const selected = given.includes(opt.id);
                  return (
                    <button
                      key={opt.id}
                      onClick={() => toggleMulti(opt.id)}
                      className={cn(
                        "flex w-full items-center gap-3 rounded-lg border px-4 py-3 text-left text-sm transition-colors",
                        selected
                          ? "border-accent bg-accent-soft text-accent-soft-foreground"
                          : "border-border text-foreground hover:border-accent/40 hover:bg-surface-secondary",
                      )}
                    >
                      <span
                        className={cn(
                          "flex size-4 shrink-0 items-center justify-center rounded-[5px] border-2",
                          selected ? "border-accent bg-accent" : "border-border",
                        )}
                      >
                        {selected && <span className="size-2 rounded-[1px] bg-white" />}
                      </span>
                      <LatexText text={opt.label} />
                    </button>
                  );
                })}

              {question.type === "isian" && (
                <input
                  type="text"
                  value={given[0] ?? ""}
                  maxLength={500}
                  onChange={(e) => setAnswer(question.id, [e.target.value])}
                  placeholder={t.answerPlaceholder}
                  className="w-full rounded-lg border border-border bg-field px-4 py-3 text-sm text-field-foreground outline-none focus:border-accent"
                />
              )}
            </div>
          </div>

          <div className="sticky bottom-0 flex items-center justify-between border-t border-border bg-surface/90 px-5 py-3 backdrop-blur-sm">
            <Button variant="ghost" size="sm" isDisabled={index === 0} onPress={() => setIndex((i) => i - 1)}>
              <ChevronLeft className="size-3.5" /> {t.previousButton}
            </Button>
            <Button variant="ghost" size="sm" isDisabled={index === questions.length - 1} onPress={() => setIndex((i) => i + 1)}>
              {t.nextButton} <ChevronRight className="size-3.5" />
            </Button>
          </div>
        </main>

        <aside className="hidden w-56 shrink-0 overflow-y-auto border-l border-border p-4 lg:block">
          <p className="mb-3 text-xs font-medium text-muted">{t.answeredCount(answered.length, questions.length)}</p>
          <div className="grid grid-cols-5 gap-2">
            {questions.map((q, i) => {
              const isAnswered = (answers[q.id]?.length ?? 0) > 0;
              const isCurrent = i === index;
              const isPending = states[q.id] === "pending";
              return (
                <button
                  key={q.id}
                  onClick={() => setIndex(i)}
                  className={cn(
                    "flex size-9 items-center justify-center rounded-md font-mono text-xs font-medium transition-colors",
                    isCurrent
                      ? "bg-gradient-ring text-white"
                      : isAnswered
                        ? "bg-accent-soft text-accent-soft-foreground"
                        : "bg-surface-secondary text-muted",
                    isPending && "ring-2 ring-warning",
                  )}
                >
                  {i + 1}
                </button>
              );
            })}
          </div>
        </aside>
      </div>

      {screenshotWarning && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-backdrop p-5">
          <div className="w-full max-w-sm rounded-xl bg-surface p-6 text-center shadow-overlay">
            <span className="mx-auto flex size-11 items-center justify-center rounded-full bg-danger-soft text-danger-soft-foreground">
              <AlertTriangle className="size-5" />
            </span>
            <p className="mt-3 text-sm font-semibold text-foreground">{t.screenshotWarningTitle}</p>
            <p className="mt-1.5 text-xs text-muted">{t.screenshotWarningBody}</p>
            <Button variant="primary" size="sm" className="mt-4 w-full" onPress={() => setScreenshotWarning(false)}>
              {t.understood}
            </Button>
          </div>
        </div>
      )}

      {fullscreenWarning && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-backdrop p-5">
          <div className="w-full max-w-sm rounded-xl bg-surface p-6 text-center shadow-overlay">
            <span className="mx-auto flex size-11 items-center justify-center rounded-full bg-danger-soft text-danger-soft-foreground">
              <AlertTriangle className="size-5" />
            </span>
            <p className="mt-3 text-sm font-semibold text-foreground">{t.fullscreenWarningTitle}</p>
            <p className="mt-1.5 text-xs text-muted">{t.fullscreenWarningBody}</p>
            <Button
              variant="primary"
              size="sm"
              className="mt-4 w-full"
              onPress={() => containerRef.current?.requestFullscreen?.().catch(() => setFullscreenWarning(false))}
            >
              <Maximize className="size-3.5" /> {t.backToFullscreen}
            </Button>
          </div>
        </div>
      )}

      <AlertDialog isOpen={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialog.Backdrop UNSTABLE_portalContainer={portalContainer ?? undefined}>
          <AlertDialog.Container>
            <AlertDialog.Dialog>
              <AlertDialog.Header>
                <AlertDialog.Icon status="warning">
                  <AlertTriangle className="size-5" />
                </AlertDialog.Icon>
                <AlertDialog.Heading>{t.confirmSubmitTitle}</AlertDialog.Heading>
              </AlertDialog.Header>
              <AlertDialog.Body>
                {t.confirmSubmitBody(answered.length, questions.length)}
                {pendingCount > 0 && <span className="mt-2 block text-warning">{t.confirmPendingWarning(pendingCount)}</span>}
              </AlertDialog.Body>
              <AlertDialog.Footer>
                <Button variant="ghost" onPress={() => setConfirmOpen(false)}>
                  {commonT.cancel}
                </Button>
                <Button variant="primary" onPress={() => void doSubmit()}>
                  {t.confirmSubmitYes}
                </Button>
              </AlertDialog.Footer>
            </AlertDialog.Dialog>
          </AlertDialog.Container>
        </AlertDialog.Backdrop>
      </AlertDialog>
    </div>
  );
}
