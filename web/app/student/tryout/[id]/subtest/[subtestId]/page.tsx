"use client";

import { use, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { Button, toast } from "@heroui/react";
import { AlertTriangle, ArrowRight, CloudOff, Maximize, WifiOff } from "lucide-react";
import { ExamTimerPill } from "@/components/countdown-ring";
import { Icon } from "@/components/icon";
import { NoteBox, Panel } from "@/components/page-blocks";
import { LatexText } from "@/components/latex";
import { QuestionTypeBadge } from "@/components/status-badge";
import { LanguageToggle } from "@/components/language-toggle";
import { api, ApiError, type RuntimeSubtest, type StudentTryout } from "@/lib/api/client";
import { useTryoutSubtestT } from "@/lib/i18n/dict/tryout-subtest";
import { useAnswerSync, useOnline, useServerClock, useViolationReporter, type InitialAnswer } from "@/lib/tryout-session";
import { cn } from "@/lib/cn";

const RESYNC_MS = 120_000;
const FLUSH_WAIT_MS = 4_000;

interface Loaded {
  title: string;
  sessionId: string;
  overview: StudentTryout;
  data: RuntimeSubtest;
  initial: InitialAnswer[];
}

export default function SubtestPage({ params }: { params: Promise<{ id: string; subtestId: string }> }) {
  const { id, subtestId } = use(params);
  const router = useRouter();
  const t = useTryoutSubtestT();
  const online = useOnline();

  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [failed, setFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [index, setIndex] = useState(0);
  const [remaining, setRemaining] = useState<number | null>(null);
  const [fullscreenWarning, setFullscreenWarning] = useState<"left" | "refused" | null>(null);
  const [screenshotWarning, setScreenshotWarning] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [deadline, setDeadline] = useState<number | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const submittedRef = useRef(false);
  const pendingRef = useRef(0);
  const expiringRef = useRef(false);
  const clock = useServerClock();

  // Where to go once this subtest is over: this attempt's own result, the next subtest's start page, or the detail.
  const leave = useCallback(
    async (finishedSessionId: string | null) => {
      if (document.fullscreenElement) document.exitFullscreen?.().catch(() => {});
      try {
        const overview = await api.runtime.overview(id);
        const done = finishedSessionId
          ? overview.attempts.find((a) => a.session_id === finishedSessionId && a.status === "completed")
          : undefined;
        if (done) {
          router.replace(`/student/tryout/${id}/result?attempt=${done.session_id}`);
          return;
        }
        const statusOf = (subtest: string) => overview.current?.subtests.find((p) => p.subtest_id === subtest)?.status ?? "pending";
        const next = overview.current ? overview.subtests.find((st) => statusOf(st.id) === "pending") : undefined;
        if (next) {
          const after = statusOf(subtestId) === "submitted" ? `?after=${subtestId}` : "";
          router.replace(`/student/tryout/${id}/subtest/${next.id}/start${after}`);
          return;
        }
      } catch {
        // The detail page re-reads the same session and shows where things stand.
      }
      router.replace(`/student/tryout/${id}`);
    },
    [id, subtestId, router],
  );

  // The server closed this subtest (deadline, or submitted elsewhere): the overview shows where things stand.
  const handleClosed = useCallback(() => {
    if (submittedRef.current) return;
    submittedRef.current = true;
    void leave(null);
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
          overview,
          sessionId,
          data,
          initial: data.questions.map((q) => ({ id: q.id, answer: q.answer, seq: q.seq })),
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status >= 400 && err.status < 500) {
          submittedRef.current = true;
          void leave(null);
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
  const reportViolation = useViolationReporter(sessionId, subtestId);
  const { answers, states, setAnswer, pendingCount, waitIdle } = useAnswerSync(
    sessionId,
    loaded?.initial ?? null,
    handleClosed,
    loadAnswer,
  );

  useEffect(() => {
    pendingRef.current = pendingCount;
  }, [pendingCount]);

  const doSubmit = useCallback(async () => {
    if (!sessionId || submittedRef.current) return;
    submittedRef.current = true;
    setSubmitting(true);
    try {
      await waitIdle(FLUSH_WAIT_MS);
      const session = await api.runtime.submitSubtest(sessionId, subtestId);
      void leave(session.status === "completed" ? session.id : null);
    } catch (err) {
      if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 401 && err.status !== 429) {
        void leave(null);
        return;
      }
      // An unknown response may still have submitted: a retry on the same session answers 409 if it did.
      submittedRef.current = false;
      setSubmitting(false);
      toast.danger(t.submitFailed);
    }
  }, [sessionId, subtestId, leave, waitIdle, t.submitFailed]);

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
        void leave(session.status === "completed" ? session.id : null);
        return;
      }
      if (part.deadline_at) setDeadline(new Date(part.deadline_at).getTime());
    } catch (err) {
      if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 401 && err.status !== 429) {
        submittedRef.current = true;
        void leave(null);
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
      if (!submittedRef.current) setFullscreenWarning("refused");
    });
    const handleChange = () => {
      if (document.fullscreenElement) {
        entered = true;
        setFullscreenWarning(null);
        return;
      }
      if (!entered || submittedRef.current) return;
      setFullscreenWarning("left");
      reportViolation("exit_fullscreen");
    };
    document.addEventListener("fullscreenchange", handleChange);
    return () => document.removeEventListener("fullscreenchange", handleChange);
  }, [loaded, reportViolation]);

  // Screenshot shortcuts: best-effort deterrent only — OS-level capture can't be reliably blocked from the browser.
  useEffect(() => {
    // Without the Fullscreen API (e.g. iPhone Safari) the tryout runs unenforced and records nothing.
    if (!loaded || typeof containerRef.current?.requestFullscreen !== "function") return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.repeat) return;
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
        <Icon icon={CloudOff} size="hero" className="text-rose" />
        <p className="font-display text-title">{t.loadErrorTitle}</p>
        <p className="max-w-sm text-caption leading-relaxed text-muted">{t.loadErrorDescription}</p>
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
        <p className="text-small text-muted">{t.loading}</p>
      </div>
    );
  }

  const subtest = loaded.data.subtest;
  const question = questions[Math.min(index, questions.length - 1)];
  const answered = questions.filter((q) => (answers[q.id]?.length ?? 0) > 0);
  const blankCount = questions.length - answered.length;
  const given = answers[question.id] ?? [];
  const isLast = index === questions.length - 1;
  const subtests = loaded.overview.subtests;
  const position = subtests.findIndex((st) => st.id === subtestId) + 1;
  const statusOf = (id: string) =>
    id === subtestId ? "running" : (loaded.overview.current?.subtests.find((p) => p.subtest_id === id)?.status ?? "pending");

  const toggleMulti = (optionId: string) => {
    setAnswer(question.id, given.includes(optionId) ? given.filter((o) => o !== optionId) : [...given, optionId]);
  };

  // Leaving mid-subtest is allowed, but never while an answer is still unsent: it would only be re-sent on return.
  const handleBack = async () => {
    await waitIdle(FLUSH_WAIT_MS);
    if (pendingRef.current > 0) {
      toast.warning(t.backBlocked(pendingRef.current));
      return;
    }
    submittedRef.current = true;
    if (document.fullscreenElement) document.exitFullscreen?.().catch(() => {});
    router.push(`/student/tryout/${id}`);
  };

  const timeText =
    remaining === null
      ? "––:––"
      : `${String(Math.floor(remaining / 60)).padStart(2, "0")}:${String(remaining % 60).padStart(2, "0")}`;
  const urgent = remaining !== null && remaining <= 60;
  const answerStatus = states[question.id] === "pending" ? "pending" : given.length > 0 ? "saved" : null;

  const optionClass = (selected: boolean) =>
    cn(
      "flex w-full items-center gap-3.5 rounded-control border px-4 py-3.5 text-left text-base transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus",
      selected ? "border-accent bg-accent-soft" : "border-border bg-surface hover:border-accent/40",
    );
  const letterBadge = (i: number) => (
    <span className="flex size-7 shrink-0 items-center justify-center rounded-chip bg-background text-label text-foreground">
      {String.fromCharCode(65 + i)}
    </span>
  );
  const dialog = "w-full max-w-sm rounded-hero bg-surface p-7 text-center shadow-dialog";

  return (
    <div ref={containerRef} className="fixed inset-0 z-50 flex flex-col bg-background">
      <header className="flex min-h-20 shrink-0 items-center gap-4 border-b border-border bg-surface px-4 lg:gap-5 lg:px-8">
        <Image src="/taruna-eduka-logo.png" alt="Taruna Eduka" width={750} height={250} className="hidden h-9 w-auto sm:block" priority />
        <div className="min-w-0 flex-1">
          <p className="truncate text-small font-semibold">{loaded.title}</p>
          <p className="mt-1 truncate text-micro text-muted">
            {subtest.name} · {t.subtestPosition(position, subtests.length)}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {pendingCount > 0 && <span className="hidden text-label text-warning sm:inline">{t.pendingCount(pendingCount)}</span>}
          <LanguageToggle />
          <ExamTimerPill role="timer" aria-label={t.timeLeft} time={timeText} urgent={urgent} />
        </div>
      </header>

      {!online && (
        <div role="alert" className="flex shrink-0 items-start gap-2.5 border-b border-warning/30 bg-warning-soft px-4 py-2.5 text-small text-warning-soft-foreground lg:px-8">
          <Icon icon={WifiOff} size="sm" className="mt-0.5" />
          <p>
            <strong className="font-semibold">{t.offlineTitle}.</strong> {t.offlineBody(pendingCount)}
          </p>
        </div>
      )}

      {confirming ? (
        <main className="flex-1 overflow-y-auto px-4 py-10 lg:py-14">
          <Panel as="section" radius="card" className="mx-auto max-w-3xl sm:p-9">
            <h1 className="font-display text-hero">{t.confirmHeading}</h1>
            <p className="mt-4 text-small leading-relaxed text-muted">{t.confirmBody(answered.length, blankCount)}</p>
            {pendingCount > 0 && <NoteBox className="mt-5" icon={AlertTriangle}>{t.confirmPendingWarning(pendingCount)}</NoteBox>}
            {!online && <NoteBox className="mt-5" icon={AlertTriangle}>{t.confirmOfflineWarning}</NoteBox>}
            <div className="mt-8 flex flex-wrap items-center gap-3.5">
              <Button variant="primary" isDisabled={submitting} onPress={() => void doSubmit()}>
                {submitting ? t.submitting : t.confirmYes} {!submitting && <Icon icon={ArrowRight} size="sm" />}
              </Button>
              <Button variant="secondary" isDisabled={submitting} onPress={() => setConfirming(false)}>
                {t.keepWorking}
              </Button>
            </div>
          </Panel>
        </main>
      ) : (
        <main className="flex-1 overflow-y-auto">
          <div className="mx-auto grid w-full max-w-7xl gap-5 px-4 py-6 lg:grid-cols-5 lg:gap-7 lg:px-8 lg:py-8">
            <div className="space-y-5 lg:col-span-4">
              <section className="min-h-96 rounded-card border border-border bg-surface p-5 sm:p-8">
                <div className="mb-5 flex items-center justify-between gap-3">
                  <span className="text-label text-muted">{t.questionProgress(index + 1, questions.length)}</span>
                  <QuestionTypeBadge type={question.type} />
                </div>

                <div className="mb-7 max-w-4xl text-lg leading-loose font-medium">
                  <LatexText text={question.content} />
                </div>

                <div className="space-y-2.5">
                  {(question.type === "pg" || question.type === "benar_salah") &&
                    question.options.map((opt, i) => {
                      const selected = given[0] === opt.id;
                      return (
                        <button key={opt.id} aria-pressed={selected} onClick={() => setAnswer(question.id, [opt.id])} className={optionClass(selected)}>
                          <span
                            className={cn(
                              "flex size-4.5 shrink-0 items-center justify-center rounded-full border-2",
                              selected ? "border-accent" : "border-border",
                            )}
                          >
                            {selected && <span className="size-2 rounded-full bg-accent" />}
                          </span>
                          {letterBadge(i)}
                          <LatexText text={opt.label} />
                        </button>
                      );
                    })}

                  {question.type === "pg_multi" &&
                    question.options.map((opt, i) => {
                      const selected = given.includes(opt.id);
                      return (
                        <button key={opt.id} aria-pressed={selected} onClick={() => toggleMulti(opt.id)} className={optionClass(selected)}>
                          <span
                            className={cn(
                              "flex size-4.5 shrink-0 items-center justify-center rounded-chip border-2",
                              selected ? "border-accent bg-accent" : "border-border",
                            )}
                          >
                            {selected && <span className="size-2 rounded-chip bg-white" />}
                          </span>
                          {letterBadge(i)}
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
                      aria-label={t.answerPlaceholder}
                      className="min-h-13.5 w-full max-w-md rounded-control border border-border bg-field px-4 py-3 text-lg text-field-foreground outline-none focus:border-accent"
                    />
                  )}
                </div>

                <p
                  role="status"
                  className={cn("mt-5 min-h-4 text-label", answerStatus === "pending" ? "font-medium text-warning" : "text-success")}
                >
                  {answerStatus === "pending" ? t.answerPending : answerStatus === "saved" ? t.answerSaved : ""}
                </p>
              </section>

              <div className="flex items-center justify-between gap-3">
                <Button variant="secondary" isDisabled={index === 0} onPress={() => setIndex((i) => i - 1)}>
                  {t.previousButton}
                </Button>
                {isLast ? (
                  <Button variant="primary" onPress={() => setConfirming(true)}>
                    {t.finishButton} <Icon icon={ArrowRight} size="sm" />
                  </Button>
                ) : (
                  <Button variant="primary" onPress={() => setIndex((i) => i + 1)}>
                    {t.nextButton} <Icon icon={ArrowRight} size="sm" />
                  </Button>
                )}
              </div>
            </div>

            <aside className="lg:col-span-1">
              <div className="rounded-card border border-border bg-surface p-5 lg:sticky lg:top-6">
                <h2 className="font-display mb-4 text-title">{t.navTitle}</h2>
                <div className="mb-4 flex flex-wrap gap-2">
                  {questions.map((q, i) => {
                    const isAnswered = (answers[q.id]?.length ?? 0) > 0;
                    const isCurrent = i === index;
                    const isPending = states[q.id] === "pending";
                    return (
                      <button
                        key={q.id}
                        onClick={() => setIndex(i)}
                        aria-label={t.questionNumberLabel(i + 1)}
                        aria-current={isCurrent ? "true" : undefined}
                        className={cn(
                          "flex size-9.5 items-center justify-center rounded-control border text-caption transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus",
                          isAnswered ? "border-accent bg-accent-soft text-accent" : "border-border bg-background",
                          isCurrent && "border-surface ring-2 ring-accent",
                          isPending && "ring-2 ring-warning",
                        )}
                      >
                        {i + 1}
                      </button>
                    );
                  })}
                </div>
                <p className="text-label text-muted">{t.answeredBlank(answered.length, blankCount)}</p>
                {pendingCount > 0 && <p className="mt-1 text-label font-medium text-warning">{t.pendingCount(pendingCount)}</p>}
                <Button variant="secondary" className="mt-5 w-full" onPress={() => setConfirming(true)}>
                  {t.submitButton} <Icon icon={ArrowRight} size="sm" />
                </Button>

                <ul className="mt-5 space-y-3 border-t border-border pt-4 text-label">
                  {subtests.map((st) => {
                    const status = statusOf(st.id);
                    return (
                      <li key={st.id} className="flex items-center justify-between gap-3">
                        <span className={cn("truncate", status === "running" ? "font-semibold text-accent" : "text-muted")}>{st.name}</span>
                        <span className={cn("shrink-0", status === "running" ? "font-semibold text-accent" : "text-muted")}>
                          {status === "running" ? t.statusRunning : status === "submitted" ? t.statusSubmitted : t.statusPending}
                        </span>
                      </li>
                    );
                  })}
                </ul>
                <Button variant="ghost" size="sm" className="mt-3 justify-start px-0 text-caption text-accent" onPress={() => void handleBack()}>
                  {t.backToDetail} <Icon icon={ArrowRight} size="xs" />
                </Button>
              </div>
            </aside>
          </div>
        </main>
      )}

      {screenshotWarning && (
        <div className="fixed inset-0 z-60 flex items-center justify-center bg-backdrop p-5">
          <div className={dialog}>
            <span className="mx-auto flex size-11 items-center justify-center rounded-full bg-danger-soft text-danger-soft-foreground">
              <Icon icon={AlertTriangle} />
            </span>
            <p className="font-display mt-4 text-title">{t.screenshotWarningTitle}</p>
            <p className="mt-2 text-caption leading-relaxed text-muted">{t.screenshotWarningBody}</p>
            <Button variant="primary" size="sm" className="mt-5 w-full" onPress={() => setScreenshotWarning(false)}>
              {t.understood}
            </Button>
          </div>
        </div>
      )}

      {fullscreenWarning && (
        <div className="fixed inset-0 z-60 flex items-center justify-center bg-backdrop p-5">
          <div className={dialog}>
            <span className="mx-auto flex size-11 items-center justify-center rounded-full bg-danger-soft text-danger-soft-foreground">
              <Icon icon={AlertTriangle} />
            </span>
            <p className="font-display mt-4 text-title">{fullscreenWarning === "left" ? t.fullscreenWarningTitle : t.fullscreenRequestTitle}</p>
            <p className="mt-2 text-caption leading-relaxed text-muted">{fullscreenWarning === "left" ? t.fullscreenWarningBody : t.fullscreenRequestBody}</p>
            <Button
              variant="primary"
              size="sm"
              className="mt-5 w-full"
              onPress={() => containerRef.current?.requestFullscreen?.().catch(() => setFullscreenWarning(null))}
            >
              <Icon icon={Maximize} size="xs" /> {t.backToFullscreen}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
