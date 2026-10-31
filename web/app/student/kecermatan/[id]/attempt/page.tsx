"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Button } from "@heroui/react";
import { CheckCircle2, ChevronRight, Clock, CloudOff, ShoppingBag, SkipForward, TriangleAlert } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { KecermatanItem } from "@/components/kecermatan-item";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, type KecermatanOverview, type KecermatanSession } from "@/lib/api/client";
import { isNoCredit, purchaseHref, useSecondsLeft } from "@/lib/kecermatan-session";
import { useServerClock } from "@/lib/tryout-session";
import { cn } from "@/lib/cn";
import { useKecermatanAttemptT } from "@/lib/i18n/dict/kecermatan-attempt";

type Answers = { kolom: number; byNumber: Record<number, string | null> };
type SendState = "sending" | "unconfirmed";
type Kolom = NonNullable<KecermatanSession["kolom"]>;
type AttemptT = ReturnType<typeof useKecermatanAttemptT>;

const SEND_RETRIES = 3;
const RETRY_MS = 1000;
const REFETCH_MS = 700;
const sendKey = (kolom: number, soal: number) => `${kolom}:${soal}`;
const mmss = (total: number) => `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;

function ExamFrame({ children }: { children: React.ReactNode }) {
  return <div className="fixed inset-0 z-50 flex flex-col bg-background">{children}</div>;
}

function ExamHeader({ title, subtitle, timer, urgent, timerLabel }: { title: string; subtitle: string; timer: string; urgent: boolean; timerLabel: string }) {
  return (
    <header className="flex h-16 shrink-0 items-center gap-4 border-b border-border px-5">
      <Image src="/taruna-eduka-logo.png" alt="Taruna Eduka" width={750} height={250} className="hidden h-8 w-auto sm:block" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-foreground">{title}</p>
        <p className="truncate text-xs text-muted">{subtitle}</p>
      </div>
      <div
        role="timer"
        aria-label={timerLabel}
        className={cn(
          "flex items-center gap-1.5 rounded-xl px-3 py-1.5 font-mono text-lg font-semibold",
          urgent ? "bg-danger-soft text-danger" : "bg-accent-soft text-accent-soft-foreground",
        )}
      >
        <Clock className="size-4" />
        {timer}
      </div>
    </header>
  );
}

function KolomScreen({
  session,
  kolom,
  byNumber,
  sends,
  remaining,
  onAnswer,
  onRetry,
  t,
}: {
  session: KecermatanSession;
  kolom: Kolom;
  byNumber: Record<number, string | null>;
  sends: Record<string, SendState>;
  remaining: number | null;
  onAnswer: (soal: number, label: string | null) => void;
  onRetry: () => void;
  t: AttemptT;
}) {
  const soalIndex = kolom.soal.findIndex((item) => !(item.number in byNumber));
  const soal = soalIndex === -1 ? null : kolom.soal[soalIndex];
  const labelOf = (label: string) => kolom.reference.find((item) => item.label === label);
  const timeUp = remaining === 0;
  const urgent = remaining !== null && remaining <= 15;
  const doneCount = kolom.soal.length - kolom.soal.filter((item) => !(item.number in byNumber)).length;
  const kolomSends = Object.entries(sends).filter(([key]) => key.startsWith(`${kolom.number}:`));
  const unconfirmed = kolomSends.filter(([, state]) => state === "unconfirmed").length;
  const sending = kolomSends.filter(([, state]) => state === "sending").length;
  const confirmed = doneCount - unconfirmed - sending;
  const labels = kolom.reference.map((item) => item.label);

  useEffect(() => {
    if (!soal || timeUp) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey || event.altKey || event.repeat) return;
      const target = event.target as HTMLElement | null;
      if (target && /INPUT|TEXTAREA|SELECT/.test(target.tagName)) return;
      const label = labels.find((item) => item.toLowerCase() === event.key.toLowerCase());
      if (!label) return;
      event.preventDefault();
      onAnswer(soal.number, label);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [soal, timeUp, labels.join(), onAnswer]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <ExamFrame>
      <ExamHeader
        title={session.test_title}
        subtitle={t.kolomProgress(kolom.number, session.kolom_count)}
        timer={remaining === null ? "--:--" : mmss(remaining)}
        urgent={urgent}
        timerLabel={t.timerLabel}
      />

      <main className="mx-auto flex min-h-0 w-full max-w-3xl flex-1 flex-col justify-center-safe overflow-y-auto px-5 py-6">
        {unconfirmed > 0 && (
          <div role="alert" className="mb-5 flex items-start gap-3 rounded-xl border border-warning bg-warning-soft p-3 text-warning-soft-foreground">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            <div className="min-w-0 flex-1 text-xs">
              <p className="font-semibold">{t.unconfirmedTitle(unconfirmed)}</p>
              <p className="mt-0.5">{t.unconfirmedBody}</p>
            </div>
            <Button size="sm" variant="secondary" onPress={onRetry}>
              {t.retrySend}
            </Button>
          </div>
        )}

        <div>
          <p className="mb-2 text-center text-xs font-medium text-muted">{t.referenceLabel(kolom.number)}</p>
          <div className="flex flex-wrap justify-center gap-2">
            {kolom.reference.map((item) => (
              <div key={item.label} className="min-w-24 rounded-lg border border-border bg-surface px-2 py-3 text-center sm:min-w-20">
                <p className="text-xs font-semibold text-muted">{item.label}</p>
                <KecermatanItem elements={item.elements} className="mt-1 text-xl font-extrabold" elementClassName="size-6 sm:size-7" />
              </div>
            ))}
          </div>
        </div>

        {soal ? (
          <>
            <div className="mt-8">
              <p className="text-center font-display text-xl font-semibold text-foreground">{t.soalPrompt}</p>
              <div className="mt-3 flex flex-wrap justify-center gap-2">
                {soal.shown.map((label) => (
                  <div key={label} className="min-w-36 rounded-lg border border-dashed border-border bg-surface px-3 py-5 text-center">
                    <KecermatanItem elements={labelOf(label)?.elements ?? []} className="text-2xl font-extrabold sm:text-3xl" elementClassName="size-6 sm:size-9" />
                  </div>
                ))}
              </div>
            </div>

            <div className="mt-8">
              <p className="mb-2 text-center text-xs font-medium text-muted">
                {t.soalProgress(soalIndex + 1, kolom.soal.length)} · {t.chooseMissingLabel}
              </p>
              <div className="flex flex-wrap justify-center gap-2">
                {kolom.reference.map((item) => (
                  <button
                    key={item.label}
                    type="button"
                    disabled={timeUp}
                    aria-label={t.answerAria(item.label)}
                    onClick={() => onAnswer(soal.number, item.label)}
                    className="flex h-16 w-24 items-center justify-center rounded-lg border border-border bg-surface font-mono text-2xl font-extrabold text-foreground transition-colors hover:border-accent hover:bg-accent-soft hover:text-accent-soft-foreground disabled:pointer-events-none disabled:opacity-50 sm:w-20"
                  >
                    {item.label}
                  </button>
                ))}
              </div>
              <div className="mt-4 flex items-center justify-center gap-4 text-xs text-muted">
                <button
                  type="button"
                  disabled={timeUp}
                  onClick={() => onAnswer(soal.number, null)}
                  className="flex items-center gap-1 font-medium text-accent hover:underline disabled:pointer-events-none disabled:opacity-50"
                >
                  <SkipForward className="size-3.5" /> {t.skip}
                </button>
                <span className="hidden sm:inline">{t.keyboardHint(labels[0] ?? "A", labels[labels.length - 1] ?? "E")}</span>
              </div>
            </div>
          </>
        ) : (
          <div className="mt-8 flex flex-col items-center text-center">
            <span className="inline-flex size-11 items-center justify-center rounded-full bg-success-soft text-success-soft-foreground">
              <CheckCircle2 className="size-5" />
            </span>
            <p className="mt-4 text-sm font-medium text-foreground">{t.allAnswered}</p>
            <p className="mt-1 text-xs text-muted">{t.waitingForTimer}</p>
          </div>
        )}

        {timeUp && <p className="mt-4 text-center text-xs font-medium text-warning-soft-foreground">{t.timeUp}</p>}

        <div className="mt-8">
          <div className="flex gap-1" aria-hidden>
            {Array.from({ length: session.kolom_count }, (_, index) => (
              <span key={index} className={cn("h-1 flex-1 rounded-full", index < kolom.number ? "bg-accent" : "bg-default")} />
            ))}
          </div>
          <p className="mt-2 text-center text-xs text-muted">{t.kolomSummary(kolom.number, session.kolom_count, doneCount)}</p>
          <p className="mt-0.5 text-center text-xs text-muted">{t.syncStatus(Math.max(0, confirmed), sending)}</p>
        </div>
      </main>
    </ExamFrame>
  );
}

export default function KecermatanAttemptPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: testId } = use(params);
  const router = useRouter();
  const t = useKecermatanAttemptT();
  const { sync: syncClock, now } = useServerClock();

  const [overview, setOverview] = useState<KecermatanOverview | null>(null);
  const [session, setSession] = useState<KecermatanSession | null>(null);
  const [answers, setAnswers] = useState<Answers>({ kolom: 0, byNumber: {} });
  const [sends, setSends] = useState<Record<string, SendState>>({});
  const [failed, setFailed] = useState(false);
  const [starting, setStarting] = useState(false);
  const [ready, setReady] = useState(false);
  const [noCredit, setNoCredit] = useState(false);
  const [startError, setStartError] = useState(false);
  const [continuing, setContinuing] = useState(false);
  const [continueFailed, setContinueFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const refetching = useRef(false);
  const alive = useRef(true);
  const submitted = useRef(new Set<string>());
  const timers = useRef(new Set<ReturnType<typeof setTimeout>>());

  useEffect(() => {
    alive.current = true;
    const pending = timers.current;
    return () => {
      alive.current = false;
      pending.forEach(clearTimeout);
    };
  }, []);

  // The server's state is the only truth: every phase change below comes from a response, never from a local timer.
  const apply = useCallback(
    (next: KecermatanSession) => {
      syncClock(next.server_time);
      setSession(next);
      if (next.phase === "completed") {
        router.replace(`/student/kecermatan/${testId}/result?attempt=${next.id}`);
        return;
      }
      if (next.kolom) {
        const stored: Record<number, string | null> = {};
        for (const item of next.kolom.answers) stored[item.soal] = item.answer;
        const number = next.kolom.number;
        // What the server holds always wins over a local guess: a stored answer is write-once.
        setAnswers((current) =>
          current.kolom === number ? { kolom: number, byNumber: { ...current.byNumber, ...stored } } : { kolom: number, byNumber: stored },
        );
        setSends((current) => {
          const kept: Record<string, SendState> = {};
          for (const [key, state] of Object.entries(current)) {
            const [kolomNo, soalNo] = key.split(":").map(Number);
            if (kolomNo === number && !(soalNo in stored)) kept[key] = state;
          }
          return kept;
        });
      }
    },
    [syncClock, router, testId],
  );

  useEffect(() => {
    let cancelled = false;
    api.kecermatanRuntime
      .overview(testId)
      .then((data) => {
        if (cancelled) return;
        setOverview(data);
        if (data.current) apply(data.current);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          router.replace(`/student/kecermatan/${testId}`);
          return;
        }
        setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [testId, apply, router, reloadKey]);

  const sessionId = session?.id ?? null;
  const refetch = useCallback(async () => {
    if (!sessionId || refetching.current) return;
    refetching.current = true;
    try {
      apply(await api.kecermatanRuntime.session(sessionId));
    } catch {
      // The next tick or focus tries again; a failed read never changes what the server decided.
    } finally {
      refetching.current = false;
    }
  }, [sessionId, apply]);

  const phase = session?.phase ?? null;
  const kolom = session?.kolom ?? null;
  const kolomLeft = useSecondsLeft(now, phase === "kolom" ? kolom?.deadline_at : null);
  const pauseLeft = useSecondsLeft(now, phase === "pause" ? session?.next_starts_at : null);

  // At zero the server has already moved on (or does within a moment): read its state instead of deciding here.
  useEffect(() => {
    if (kolomLeft !== 0 && pauseLeft !== 0) return;
    const first = setTimeout(() => void refetch(), 0);
    const interval = setInterval(() => void refetch(), REFETCH_MS);
    return () => {
      clearTimeout(first);
      clearInterval(interval);
    };
  }, [kolomLeft, pauseLeft, refetch]);

  useEffect(() => {
    if (!sessionId) return;
    const onFocus = () => void refetch();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [sessionId, refetch]);

  const setSend = useCallback((key: string, state: SendState | null) => {
    setSends((current) => {
      const next = { ...current };
      if (state) next[key] = state;
      else delete next[key];
      return next;
    });
  }, []);

  // Only a 200 ack counts as saved. A 409 means the server already decided, so it is re-read rather than retried.
  const send = useCallback(
    (kolomNo: number, soalNo: number, label: string | null) => {
      if (!sessionId) return;
      const key = sendKey(kolomNo, soalNo);
      const attempt = (n: number) => {
        setSend(key, "sending");
        api.kecermatanRuntime
          .answer(sessionId, kolomNo, soalNo, label)
          .then((ack) => {
            if (!alive.current) return;
            setSend(key, null);
            if (ack.answer !== label) {
              setAnswers((current) =>
                current.kolom === kolomNo ? { ...current, byNumber: { ...current.byNumber, [soalNo]: ack.answer } } : current,
              );
            }
          })
          .catch((err: unknown) => {
            if (!alive.current) return;
            if (err instanceof ApiError && err.status === 409) {
              setSend(key, null);
              if (err.code !== "kecermatan_answer_locked") {
                // The server did not record it; show that instead of a local answer it never took.
                submitted.current.delete(key);
                setAnswers((current) => {
                  if (current.kolom !== kolomNo) return current;
                  const byNumber = { ...current.byNumber };
                  delete byNumber[soalNo];
                  return { ...current, byNumber };
                });
              }
              void refetch();
              return;
            }
            if (n < SEND_RETRIES) {
              const timer = setTimeout(() => {
                timers.current.delete(timer);
                attempt(n + 1);
              }, RETRY_MS);
              timers.current.add(timer);
              return;
            }
            setSend(key, "unconfirmed");
          });
      };
      attempt(0);
    },
    [sessionId, refetch, setSend],
  );

  useEffect(() => {
    const onOnline = () => {
      void refetch();
    };
    window.addEventListener("online", onOnline);
    return () => window.removeEventListener("online", onOnline);
  }, [refetch]);

  const start = async () => {
    setStarting(true);
    setStartError(false);
    try {
      apply(await api.kecermatanRuntime.start(testId));
    } catch (err) {
      if (isNoCredit(err)) setNoCredit(true);
      else setStartError(true);
    } finally {
      setStarting(false);
    }
  };

  const goNext = async () => {
    if (!sessionId || continuing) return;
    setContinuing(true);
    setContinueFailed(false);
    try {
      apply(await api.kecermatanRuntime.next(sessionId));
    } catch {
      setContinueFailed(true);
      void refetch();
    } finally {
      setContinuing(false);
    }
  };

  const kolomNumber = kolom?.number ?? null;
  const answer = useCallback(
    (soalNo: number, label: string | null) => {
      if (kolomNumber === null) return;
      const key = sendKey(kolomNumber, soalNo);
      if (submitted.current.has(key)) return;
      submitted.current.add(key);
      setAnswers((current) => (current.kolom === kolomNumber ? { ...current, byNumber: { ...current.byNumber, [soalNo]: label } } : current));
      send(kolomNumber, soalNo, label);
    },
    [kolomNumber, send],
  );

  const retryUnconfirmed = () => {
    if (kolomNumber === null) return;
    for (const [key, state] of Object.entries(sends)) {
      if (state !== "unconfirmed") continue;
      const [kolomNo, soalNo] = key.split(":").map(Number);
      if (kolomNo !== kolomNumber || !(soalNo in answers.byNumber)) continue;
      send(kolomNo, soalNo, answers.byNumber[soalNo]);
    }
  };

  if (failed) {
    return (
      <ExamFrame>
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center">
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
      </ExamFrame>
    );
  }

  if (!overview) {
    return <ExamFrame><div className="flex flex-1 items-center justify-center text-sm text-muted">{t.loadingSoal}</div></ExamFrame>;
  }

  const test = overview.test;

  if (!session) {
    const left = test.attempts_remaining;
    const blocked = noCredit || left < 1;
    const lastLabel = String.fromCharCode(64 + test.options_per_soal);
    const rules = [
      t.rulesReference(test.options_per_soal, lastLabel),
      t.rulesMissing,
      t.rulesWriteOnce,
      t.rulesTimer(test.kolom_count, Math.round(test.kolom_duration_seconds / 60)),
      t.rulesBlank,
      t.rulesPause,
      t.rulesCredit,
    ];
    return (
      <AppShell title={t.pageTitle} description={test.title}>
        <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
          <section className="rounded-2xl border border-border bg-surface p-5">
            <p className="text-xs font-medium tracking-wide text-muted uppercase">{t.eyebrow}</p>
            <h2 className="font-display mt-1 text-xl font-semibold text-foreground">{t.readinessHeading}</h2>
            <ul className="mt-4 space-y-2.5 text-sm text-muted">
              {rules.map((rule) => (
                <li key={rule} className="flex gap-2.5">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-accent" />
                  <span>{rule}</span>
                </li>
              ))}
            </ul>
          </section>

          <section className="h-fit rounded-2xl border border-border bg-surface p-5">
            <h2 className="font-display text-lg font-semibold text-foreground">{t.checkTitle}</h2>
            <p className="mt-2 text-sm text-muted">{t.attemptsLeft(Math.max(0, left))}</p>
            {blocked ? (
              <div className="mt-4 space-y-3">
                <p className="text-sm font-medium text-foreground">{t.noCreditTitle}</p>
                <p className="text-xs text-muted">{t.noCreditDescription}</p>
                <LinkButton href={purchaseHref(overview.product_id)} variant="primary" className="w-full">
                  <ShoppingBag className="size-4" /> {t.buyMore}
                </LinkButton>
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                <label className="flex items-start gap-2.5 text-sm text-foreground">
                  <input type="checkbox" checked={ready} onChange={(event) => setReady(event.target.checked)} className="mt-0.5 size-4 accent-accent" />
                  {t.readyCheck}
                </label>
                {startError && <p role="alert" className="text-xs text-danger">{t.startError}</p>}
                <Button variant="primary" size="lg" className="w-full" isDisabled={!ready || starting} onPress={() => void start()}>
                  {starting ? t.starting : t.startKolom1} <ChevronRight className="size-4" />
                </Button>
              </div>
            )}
            <LinkButton href={`/student/kecermatan/${testId}`} variant="ghost" size="sm" className="mt-3 w-full">
              {t.backToDetail}
            </LinkButton>
          </section>
        </div>
      </AppShell>
    );
  }

  if (session.phase === "pause" && session.next_kolom !== null) {
    const nextKolom = session.next_kolom;
    const startingNow = pauseLeft === 0;
    return (
      <ExamFrame>
        <ExamHeader
          title={session.test_title}
          subtitle={t.kolomFinished(session.finished_kolom ?? nextKolom - 1)}
          timer={pauseLeft === null ? "--:--" : mmss(pauseLeft)}
          urgent={false}
          timerLabel={t.timerLabel}
        />
        <main className="flex min-h-0 flex-1 flex-col items-center justify-center-safe overflow-y-auto px-5 py-6">
          <div className="w-full max-w-sm rounded-2xl border border-border bg-surface p-6 text-center">
            <p className="text-sm font-medium text-muted">{t.kolomFinished(session.finished_kolom ?? nextKolom - 1)}</p>
            <h1 className="font-display mt-1 text-2xl font-semibold text-foreground">{t.pauseHeading}</h1>
            <p className="mt-2 text-sm text-muted">{startingNow ? t.startingNext : t.pauseHint(nextKolom)}</p>
            <p className="font-display mt-4 text-5xl font-semibold text-accent tabular-nums">{pauseLeft ?? "—"}</p>
            <Button variant="primary" size="lg" className="mt-5 w-full" isDisabled={continuing || startingNow} onPress={() => void goNext()}>
              {t.continueToKolom(nextKolom)} <ChevronRight className="size-4" />
            </Button>
            {continueFailed && <p role="alert" className="mt-3 text-xs text-danger">{t.continueFailed}</p>}
            <p className="mt-4 text-xs text-muted">{t.pauseNote}</p>
          </div>
        </main>
      </ExamFrame>
    );
  }

  if (!kolom) {
    return <ExamFrame><div className="flex flex-1 items-center justify-center text-sm text-muted">{t.loadingSoal}</div></ExamFrame>;
  }

  return (
    <KolomScreen
      session={session}
      kolom={kolom}
      byNumber={answers.kolom === kolom.number ? answers.byNumber : {}}
      sends={sends}
      remaining={kolomLeft}
      onAnswer={answer}
      onRetry={retryUnconfirmed}
      t={t}
    />
  );
}
