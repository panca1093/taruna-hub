"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { notFound, useRouter } from "next/navigation";
import { AlertDialog, Button } from "@heroui/react";
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Maximize,
  Send,
} from "lucide-react";
import { CountdownRing } from "@/components/countdown-ring";
import { LatexText } from "@/components/latex";
import { QuestionTypeBadge } from "@/components/status-badge";
import { LanguageToggle } from "@/components/language-toggle";
import { useUser } from "@/lib/session-context";
import { useCommonT } from "@/lib/i18n/dict/common";
import { useTryoutSubtestT } from "@/lib/i18n/dict/tryout-subtest";
import {
  completeSubtest,
  logViolation,
  saveAnswer,
  startSubtest,
  type TryoutSessionState,
} from "@/lib/tryout-session";
import { getQuestionsForSubtest, getSubtest, getTryout } from "@/lib/mock-data";
import { cn } from "@/lib/cn";
import { useHydrated } from "@/lib/use-hydrated";

export default function SubtestPage({
  params,
}: {
  params: Promise<{ id: string; subtestId: string }>;
}) {
  const { id, subtestId } = use(params);
  const router = useRouter();
  const user = useUser();
  const t = useTryoutSubtestT();
  const commonT = useCommonT();

  const tryout = getTryout(id);
  const subtest = getSubtest(subtestId);
  const questions = subtest ? getQuestionsForSubtest(subtest.id) : [];

  const [session, setSession] = useState<TryoutSessionState | null>(null);
  const [index, setIndex] = useState(0);
  const [remaining, setRemaining] = useState<number | null>(null);
  const [fullscreenWarning, setFullscreenWarning] = useState(false);
  const [screenshotWarning, setScreenshotWarning] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [portalContainer, setPortalContainer] = useState<HTMLDivElement | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const submittedRef = useRef(false);

  const found = !!tryout && !!subtest;

  const doSubmit = useCallback(() => {
    if (!found || submittedRef.current) return;
    submittedRef.current = true;
    if (document.fullscreenElement) document.exitFullscreen?.().catch(() => {});
    const updated = completeSubtest(id, user.id, subtestId);
    if (updated.status === "completed") {
      router.replace(`/student/tryout/${id}/result`);
    } else {
      router.replace(`/student/tryout/${id}`);
    }
  }, [found, id, subtestId, user.id, router]);

  // Init session + guard against re-entering a completed subtest.
  const hydrated = useHydrated();
  const seedKey = `${id}:${subtestId}:${user.id}`;
  const [seededKey, setSeededKey] = useState<string | null>(null);
  const [alreadyCompleted, setAlreadyCompleted] = useState(false);

  if (hydrated && found && seededKey !== seedKey) {
    const s = startSubtest(id, user.id, subtestId);
    const completed = s.subtestStatus[subtestId] === "completed";
    setSeededKey(seedKey);
    setAlreadyCompleted(completed);
    setSession(completed ? null : s);
  }

  useEffect(() => {
    if (alreadyCompleted) router.replace(`/student/tryout/${id}`);
  }, [alreadyCompleted, router, id]);

  // Countdown timer, resilient to reloads via persisted subtestStartedAt.
  useEffect(() => {
    if (!tryout || !subtest || !session) return;
    const startedAt = session.subtestStartedAt[subtest.id] ?? Date.now();
    const tick = () => {
      const elapsed = (Date.now() - startedAt) / 1000;
      const left = Math.max(0, Math.round(subtest.durationSeconds - elapsed));
      setRemaining(left);
      if (left <= 0) doSubmit();
    };
    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, [tryout, subtest, session, doSubmit]);

  // Fullscreen: request on entry, track exits as violations.
  useEffect(() => {
    if (!tryout || !subtest) return;
    containerRef.current?.requestFullscreen?.().catch(() => {});

    const handleChange = () => {
      if (!document.fullscreenElement && !submittedRef.current) {
        setFullscreenWarning(true);
        setSession(logViolation(tryout.id, user.id));
      } else {
        setFullscreenWarning(false);
      }
    };
    document.addEventListener("fullscreenchange", handleChange);
    return () => document.removeEventListener("fullscreenchange", handleChange);
  }, [tryout, subtest, user.id]);

  // Screenshot shortcuts: best-effort deterrent only — OS-level capture
  // (e.g. the screenshot utility itself) can't be reliably blocked from the browser.
  useEffect(() => {
    if (!tryout || !subtest) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      const isPrintScreen = e.key === "PrintScreen";
      const isMacCapture = e.metaKey && e.shiftKey && ["3", "4", "5"].includes(e.key);
      const isWinSnip = e.metaKey && e.shiftKey && e.key.toLowerCase() === "s";
      if (!isPrintScreen && !isMacCapture && !isWinSnip) return;
      e.preventDefault();
      setScreenshotWarning(true);
      setSession(logViolation(tryout.id, user.id, "screenshot_attempt"));
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [tryout, subtest, user.id]);

  if (!tryout || !subtest) notFound();

  const question = questions[index];
  const answered = session ? Object.keys(session.answers).filter((qid) => questions.some((q) => q.id === qid)) : [];

  const setAnswer = (value: string[]) => {
    if (!tryout) return;
    setSession(saveAnswer(tryout.id, user.id, question.id, value));
  };

  const toggleMulti = (optionId: string) => {
    const current = session?.answers[question.id] ?? [];
    const next = current.includes(optionId) ? current.filter((o) => o !== optionId) : [...current, optionId];
    setAnswer(next);
  };

  const minutes = remaining !== null ? Math.floor(remaining / 60) : 0;
  const seconds = remaining !== null ? remaining % 60 : 0;
  const urgent = remaining !== null && remaining <= 60;
  const percentLeft = remaining !== null ? (remaining / subtest.durationSeconds) * 100 : 100;

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
          <p className="truncate text-xs text-muted">{tryout.title}</p>
        </div>
        <div className="flex items-center gap-3">
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
              <span className="text-xs font-medium text-muted">
                {t.questionProgress(index + 1, questions.length)}
              </span>
              <QuestionTypeBadge type={question.type} />
            </div>

            <div className="text-[15px] leading-relaxed text-foreground">
              <LatexText text={question.content} />
            </div>

            <div className="mt-6 space-y-2.5">
              {(question.type === "pg" || question.type === "benar_salah") &&
                question.options?.map((opt) => {
                  const selected = session?.answers[question.id]?.[0] === opt.id;
                  return (
                    <button
                      key={opt.id}
                      onClick={() => setAnswer([opt.id])}
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
                question.options?.map((opt) => {
                  const selected = session?.answers[question.id]?.includes(opt.id) ?? false;
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
                  value={session?.answers[question.id]?.[0] ?? ""}
                  onChange={(e) => setAnswer([e.target.value])}
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
            <Button
              variant="ghost"
              size="sm"
              isDisabled={index === questions.length - 1}
              onPress={() => setIndex((i) => i + 1)}
            >
              {t.nextButton} <ChevronRight className="size-3.5" />
            </Button>
          </div>
        </main>

        <aside className="hidden w-56 shrink-0 overflow-y-auto border-l border-border p-4 lg:block">
          <p className="mb-3 text-xs font-medium text-muted">
            {t.answeredCount(answered.length, questions.length)}
          </p>
          <div className="grid grid-cols-5 gap-2">
            {questions.map((q, i) => {
              const isAnswered = (session?.answers[q.id]?.length ?? 0) > 0;
              const isCurrent = i === index;
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
            <p className="mt-1.5 text-xs text-muted">
              {t.screenshotWarningBody}
            </p>
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
            <p className="mt-1.5 text-xs text-muted">
              {t.fullscreenWarningBody}
            </p>
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
              </AlertDialog.Body>
              <AlertDialog.Footer>
                <Button variant="ghost" onPress={() => setConfirmOpen(false)}>
                  {commonT.cancel}
                </Button>
                <Button variant="primary" onPress={doSubmit}>
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
