"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@heroui/react";
import { CheckCircle2, ChevronRight, CloudOff, SkipForward } from "lucide-react";
import { CountdownRing } from "@/components/countdown-ring";
import { KecermatanItem } from "@/components/kecermatan-item";
import { ApiError, api, type KecermatanOverview, type KecermatanSession } from "@/lib/api/client";
import { isNoCredit, purchaseHref, useSecondsLeft } from "@/lib/kecermatan-session";
import { useServerClock } from "@/lib/tryout-session";
import { cn } from "@/lib/cn";
import { useKecermatanAttemptT } from "@/lib/i18n/dict/kecermatan-attempt";

type Answers = { kolom: number; byNumber: Record<number, string | null> };

const SEND_RETRIES = 3;
const RETRY_MS = 1000;
const REFETCH_MS = 700;

export default function KecermatanAttemptPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: testId } = use(params);
  const router = useRouter();
  const t = useKecermatanAttemptT();
  const clock = useServerClock();

  const [overview, setOverview] = useState<KecermatanOverview | null>(null);
  const [session, setSession] = useState<KecermatanSession | null>(null);
  const [answers, setAnswers] = useState<Answers>({ kolom: 0, byNumber: {} });
  const [failed, setFailed] = useState(false);
  const [starting, setStarting] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const refetching = useRef(false);

  // The server's state is the only truth: every phase change below comes from a response, never from a local timer.
  const apply = useCallback(
    (next: KecermatanSession) => {
      clock.sync(next.server_time);
      setSession(next);
      if (next.phase === "completed") {
        router.replace(`/student/kecermatan/${testId}/result?attempt=${next.id}`);
        return;
      }
      if (next.kolom) {
        const stored: Record<number, string | null> = {};
        for (const item of next.kolom.answers) stored[item.soal] = item.answer;
        const number = next.kolom.number;
        setAnswers((current) => (current.kolom === number ? { kolom: number, byNumber: { ...stored, ...current.byNumber } } : { kolom: number, byNumber: stored }));
      }
    },
    [clock, router, testId],
  );

  useEffect(() => {
    let cancelled = false;
    api.kecermatanRuntime
      .overview(testId)
      .then((data) => {
        if (cancelled) return;
        setOverview(data);
        if (data.current) {
          apply(data.current);
          return;
        }
        if (data.test.attempts_remaining < 1) router.replace(purchaseHref(data.product_id));
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
  const kolomLeft = useSecondsLeft(clock.now, phase === "kolom" ? kolom?.deadline_at : null);
  const pauseLeft = useSecondsLeft(clock.now, phase === "pause" ? session?.next_starts_at : null);

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

  const start = async () => {
    setStarting(true);
    try {
      apply(await api.kecermatanRuntime.start(testId));
    } catch (err) {
      if (isNoCredit(err)) {
        router.replace(purchaseHref(overview?.product_id));
        return;
      }
      setFailed(true);
    } finally {
      setStarting(false);
    }
  };

  const goNext = async () => {
    if (!sessionId) return;
    try {
      apply(await api.kecermatanRuntime.next(sessionId));
    } catch {
      void refetch();
    }
  };

  const send = (number: number, soalNumber: number, label: string | null, attempt = 0) => {
    if (!sessionId) return;
    api.kecermatanRuntime.answer(sessionId, number, soalNumber, label).catch((err: unknown) => {
      if (err instanceof ApiError && err.status === 409) {
        if (err.code !== "kecermatan_answer_locked") void refetch();
        return;
      }
      if (attempt < SEND_RETRIES) setTimeout(() => send(number, soalNumber, label, attempt + 1), RETRY_MS);
    });
  };

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

  if (!overview) {
    return <div className="fixed inset-0 z-50 flex items-center justify-center bg-background text-sm text-muted">{t.loadingSoal}</div>;
  }

  const test = overview.test;

  if (!session) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-background p-5">
        <div className="w-full max-w-md text-center">
          <p className="text-xs font-medium tracking-wide text-muted uppercase">{t.eyebrow}</p>
          <h1 className="font-display mt-1 text-xl font-semibold text-foreground">{test.title}</h1>
          <p className="mt-3 text-sm leading-relaxed text-muted">
            {t.instructionsPart1(test.options_per_soal, String.fromCharCode(64 + test.options_per_soal))}{" "}
            {t.instructionsPart2(test.soal_per_kolom)} <strong className="text-foreground">{t.instructionsBold}</strong>{" "}
            {t.instructionsPart3}
          </p>
          <div className="mt-5 rounded-xl border border-border bg-surface-secondary p-4 text-left text-xs text-muted">
            <p>
              <strong className="text-foreground">{t.kolomTimerBold(test.kolom_count)}</strong>{" "}
              {t.kolomTimerRest(Math.round(test.kolom_duration_seconds / 60))}
            </p>
            <p className="mt-1.5">{t.blankNotWrong}</p>
          </div>
          <Button variant="primary" size="lg" className="mt-6 w-full" isDisabled={starting} onPress={() => void start()}>
            {t.startKolom1} <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>
    );
  }

  if (session.phase === "pause" && session.next_kolom !== null) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-background p-5">
        <div className="w-full max-w-sm text-center">
          <p className="text-sm font-medium text-muted">{t.kolomFinished(session.finished_kolom ?? session.next_kolom - 1)}</p>
          <p className="font-display mt-1 text-3xl font-semibold text-foreground">{t.continueToKolom(session.next_kolom)}</p>
          <p className="mt-4 font-mono text-2xl font-semibold text-accent">{pauseLeft ?? session.pause_seconds}</p>
          <Button variant="primary" size="lg" className="mt-5 w-full" onPress={() => void goNext()}>
            {t.continueNow} <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>
    );
  }

  if (!kolom) {
    return <div className="fixed inset-0 z-50 flex items-center justify-center bg-background text-sm text-muted">{t.loadingSoal}</div>;
  }

  const remaining = kolomLeft ?? session.kolom_duration_seconds;
  const minutes = Math.floor(remaining / 60);
  const seconds = remaining % 60;
  const urgent = remaining <= 15;
  const percentLeft = (remaining / session.kolom_duration_seconds) * 100;
  const byNumber = answers.kolom === kolom.number ? answers.byNumber : {};
  const soalIndex = kolom.soal.findIndex((item) => !(item.number in byNumber));
  const soal = soalIndex === -1 ? null : kolom.soal[soalIndex];
  const labelOf = (label: string) => kolom.reference.find((item) => item.label === label);

  // Answering the last soal doesn't jump to the next kolom early: the kolom only ends at its server deadline.
  const answer = (label: string | null) => {
    if (!soal) return;
    setAnswers({ kolom: kolom.number, byNumber: { ...byNumber, [soal.number]: label } });
    send(kolom.number, soal.number, label);
  };

  const timerHeader = (
    <header className="flex h-16 shrink-0 items-center gap-4 border-b border-border px-5">
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-foreground">{session.test_title}</p>
        <p className="truncate text-xs text-muted">{t.kolomProgress(kolom.number, session.kolom_count)}</p>
      </div>
      <CountdownRing value={percentLeft} size={40} strokeWidth={4} urgent={urgent} animate={false}>
        <span className={cn("font-mono text-[11px] font-semibold", urgent ? "text-danger" : "text-foreground")}>
          {String(minutes).padStart(2, "0")}:{String(seconds).padStart(2, "0")}
        </span>
      </CountdownRing>
    </header>
  );

  if (!soal) {
    return (
      <div className="fixed inset-0 z-50 flex flex-col bg-background">
        {timerHeader}
        <main className="mx-auto flex w-full max-w-md flex-1 flex-col items-center justify-center px-5 py-6 text-center">
          <span className="inline-flex size-11 items-center justify-center rounded-full bg-success-soft text-success-soft-foreground">
            <CheckCircle2 className="size-5" />
          </span>
          <p className="mt-4 text-sm font-medium text-foreground">{t.allAnswered}</p>
          <p className="mt-1 text-xs text-muted">{t.waitingForTimer}</p>
        </main>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-background">
      {timerHeader}

      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col justify-center px-5 py-6">
        <div>
          <p className="mb-2 text-center text-xs font-medium tracking-wide text-muted uppercase">{t.referenceLabel}</p>
          <div className="flex flex-wrap justify-center gap-2">
            {kolom.reference.map((item) => (
              <div key={item.label} className="w-24 rounded-lg border border-border bg-surface-secondary px-1 py-3 text-center sm:w-20">
                <p className="text-xs font-semibold text-muted">{item.label}</p>
                <KecermatanItem elements={item.elements} className="mt-1 text-xl font-extrabold" elementClassName="size-7" />
              </div>
            ))}
          </div>
        </div>

        <div className="mt-8">
          <div className="mb-2 flex items-center justify-center gap-4">
            <p className="text-xs font-medium text-muted">{t.soalProgress(soalIndex + 1, kolom.soal.length)}</p>
            <button type="button" onClick={() => answer(null)} className="flex items-center gap-1 text-xs font-medium text-muted hover:text-foreground">
              <SkipForward className="size-3.5" /> {t.skip}
            </button>
          </div>
          <p className="text-center text-base text-foreground">{t.soalPrompt}</p>
          <div className="mt-3 flex flex-wrap justify-center gap-2">
            {soal.shown.map((label) => (
              <div key={label} className="w-36 rounded-lg border border-dashed border-border px-1 py-5 text-center">
                <KecermatanItem elements={labelOf(label)?.elements ?? []} className="text-2xl font-extrabold sm:text-3xl" elementClassName="size-9" />
              </div>
            ))}
          </div>
        </div>

        <div className="mt-8">
          <p className="mb-2 text-center text-xs font-medium text-muted">{t.chooseMissingLabel}</p>
          <div className="flex flex-wrap justify-center gap-2">
            {kolom.reference.map((item) => (
              <button
                key={item.label}
                onClick={() => answer(item.label)}
                className="flex h-16 w-24 items-center justify-center rounded-lg border sm:w-20 border-border font-mono text-2xl font-extrabold text-foreground transition-colors hover:border-accent hover:bg-accent-soft hover:text-accent-soft-foreground"
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>
      </main>
    </div>
  );
}
