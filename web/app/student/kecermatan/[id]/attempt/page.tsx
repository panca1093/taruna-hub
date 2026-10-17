"use client";

import { use, useCallback, useEffect, useState } from "react";
import { notFound, useRouter } from "next/navigation";
import { Button } from "@heroui/react";
import { CheckCircle2, ChevronRight, SkipForward } from "lucide-react";
import { CountdownRing } from "@/components/countdown-ring";
import { useUser } from "@/lib/session-context";
import {
  answerSoal,
  completeKolom,
  initSession,
  startKolom,
  type KecermatanSessionState,
} from "@/lib/kecermatan-session";
import { getKecermatanTest } from "@/lib/mock-data";
import { cn } from "@/lib/cn";
import { useKecermatanAttemptT } from "@/lib/i18n/dict/kecermatan-attempt";
import { useHydrated } from "@/lib/use-hydrated";

type Phase = "instructions" | "kolom" | "jeda";

export default function KecermatanAttemptPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const user = useUser();
  const test = getKecermatanTest(id);
  const t = useKecermatanAttemptT();

  const [session, setSession] = useState<KecermatanSessionState | null>(null);
  const [phase, setPhase] = useState<Phase>("instructions");
  const [soalIndex, setSoalIndex] = useState(0);
  const [remaining, setRemaining] = useState<number | null>(null);
  const [jedaLeft, setJedaLeft] = useState(5);
  const [nextKolomNumber, setNextKolomNumber] = useState<number | null>(null);

  // Init: resume mid-attempt if a kolom already started, otherwise show instructions first.
  const hydrated = useHydrated();
  const seedKey = `${id}:${user.id}`;
  const [seededKey, setSeededKey] = useState<string | null>(null);
  const [alreadyCompleted, setAlreadyCompleted] = useState(false);

  if (hydrated && test && seededKey !== seedKey) {
    setSeededKey(seedKey);
    const s = initSession(test.id, user.id);
    if (s.status === "completed") {
      setAlreadyCompleted(true);
    } else if (Object.keys(s.kolomStartedAt).length > 0) {
      const started = startKolom(test.id, user.id, s.currentKolom);
      setSession(started);
      const soal = started.soalByKolom[s.currentKolom] ?? [];
      const firstPending = soal.findIndex((q) => q.selectedLabel === null);
      setSoalIndex(firstPending === -1 ? soal.length : firstPending);
      setPhase("kolom");
    } else {
      setSession(s);
    }
  }

  useEffect(() => {
    if (alreadyCompleted) router.replace(`/student/kecermatan/${id}/result`);
  }, [alreadyCompleted, router, id]);

  const hasTest = !!test;

  const beginKolom = useCallback(
    (kolomNumber: number) => {
      if (!hasTest) return;
      const s = startKolom(id, user.id, kolomNumber);
      setSession(s);
      setSoalIndex(0);
      setPhase("kolom");
    },
    [hasTest, id, user.id, setSession, setSoalIndex, setPhase],
  );

  const finishKolom = useCallback(
    (kolomNumber: number) => {
      if (!hasTest) return;
      const s = completeKolom(id, user.id, kolomNumber);
      setSession(s);
      if (s.status === "completed") {
        router.replace(`/student/kecermatan/${id}/result`);
        return;
      }
      setNextKolomNumber(kolomNumber + 1);
      setJedaLeft(5);
      setPhase("jeda");
    },
    [hasTest, id, user.id, router, setSession, setNextKolomNumber, setJedaLeft, setPhase],
  );

  // Countdown timer for the active kolom, resilient to reload via persisted kolomStartedAt.
  useEffect(() => {
    if (!test || !session || phase !== "kolom") return;
    const kolomNumber = session.currentKolom;
    const startedAt = session.kolomStartedAt[kolomNumber] ?? Date.now();
    const tick = () => {
      const elapsed = (Date.now() - startedAt) / 1000;
      const left = Math.max(0, Math.round(test.kolomDurationSeconds - elapsed));
      setRemaining(left);
      if (left <= 0) finishKolom(kolomNumber);
    };
    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [test, session?.currentKolom, phase]);

  // Jeda auto-advance countdown.
  useEffect(() => {
    if (phase !== "jeda" || nextKolomNumber === null) return;
    const t = setTimeout(() => (jedaLeft <= 1 ? beginKolom(nextKolomNumber) : setJedaLeft((v) => v - 1)), 1000);
    return () => clearTimeout(t);
  }, [phase, jedaLeft, nextKolomNumber, beginKolom]);

  if (!test) notFound();

  const kolomNumber = session?.currentKolom ?? 1;
  const soalList = session?.soalByKolom[kolomNumber] ?? [];
  const soal = soalList[soalIndex];

  // Answering the last soal doesn't jump to the next kolom early — the kolom only
  // advances when its timer runs out, so students can't skip ahead of the clock.
  const answer = (label: string | null) => {
    if (!session || !soal) return;
    const s = answerSoal(test.id, user.id, kolomNumber, soal.soalNumber, label);
    setSession(s);
    setSoalIndex((i) => i + 1);
  };

  const minutes = remaining !== null ? Math.floor(remaining / 60) : 0;
  const seconds = remaining !== null ? remaining % 60 : 0;
  const urgent = remaining !== null && remaining <= 15;
  const percentLeft = remaining !== null ? (remaining / test.kolomDurationSeconds) * 100 : 100;

  if (phase === "instructions") {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-background p-5">
        <div className="w-full max-w-md text-center">
          <p className="text-xs font-medium tracking-wide text-muted uppercase">{t.eyebrow}</p>
          <h1 className="font-display mt-1 text-xl font-semibold text-foreground">{test.title}</h1>
          <p className="mt-3 text-sm leading-relaxed text-muted">
            {t.instructionsPart1(test.optionsPerSoal, String.fromCharCode(64 + test.optionsPerSoal))}{" "}
            {t.instructionsPart2(test.soalPerKolom)}{" "}
            <strong className="text-foreground">{t.instructionsBold}</strong>{" "}
            {t.instructionsPart3}
          </p>
          <div className="mt-5 rounded-xl border border-border bg-surface-secondary p-4 text-left text-xs text-muted">
            <p>
              <strong className="text-foreground">{t.kolomTimerBold(test.kolomCount)}</strong>{" "}
              {t.kolomTimerRest(Math.round(test.kolomDurationSeconds / 60))}
            </p>
            <p className="mt-1.5">{t.blankNotWrong}</p>
          </div>
          <Button variant="primary" size="lg" className="mt-6 w-full" onPress={() => beginKolom(1)}>
            {t.startKolom1} <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>
    );
  }

  if (phase === "jeda") {
    const justFinished = nextKolomNumber !== null ? nextKolomNumber - 1 : kolomNumber;
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-background p-5">
        <div className="w-full max-w-sm text-center">
          <p className="text-sm font-medium text-muted">{t.kolomFinished(justFinished)}</p>
          <p className="font-display mt-1 text-3xl font-semibold text-foreground">
            {nextKolomNumber !== null && t.continueToKolom(nextKolomNumber)}
          </p>
          <p className="mt-4 font-mono text-2xl font-semibold text-accent">{jedaLeft}</p>
          <Button
            variant="primary"
            size="lg"
            className="mt-5 w-full"
            onPress={() => nextKolomNumber !== null && beginKolom(nextKolomNumber)}
          >
            {t.continueNow} <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>
    );
  }

  const timerHeader = (
    <header className="flex h-16 shrink-0 items-center gap-4 border-b border-border px-5">
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-foreground">{test.title}</p>
        <p className="truncate text-xs text-muted">
          {t.kolomProgress(kolomNumber, test.kolomCount)}
        </p>
      </div>
      <CountdownRing value={percentLeft} size={40} strokeWidth={4} urgent={urgent} animate={false}>
        <span className={cn("font-mono text-[11px] font-semibold", urgent ? "text-danger" : "text-foreground")}>
          {String(minutes).padStart(2, "0")}:{String(seconds).padStart(2, "0")}
        </span>
      </CountdownRing>
    </header>
  );

  // All soal in this kolom answered, but the kolom only advances when its timer
  // expires — no skipping ahead of the clock.
  if (session && soalIndex >= soalList.length) {
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

  if (!soal) {
    return <div className="fixed inset-0 z-50 flex items-center justify-center bg-background text-sm text-muted">{t.loadingSoal}</div>;
  }

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-background">
      {timerHeader}

      <main className="mx-auto flex w-full max-w-xl flex-1 flex-col justify-center px-5 py-6">
        <div>
          <p className="mb-2 text-xs font-medium tracking-wide text-muted uppercase">{t.referenceLabel}</p>
          <div className="grid grid-cols-5 gap-2">
            {soal.referenceSet.map((item) => (
              <div key={item.label} className="rounded-lg border border-border bg-surface-secondary py-2.5 text-center">
                <p className="text-[10px] font-medium text-muted">{item.label}</p>
                <p className="font-mono text-sm font-semibold tracking-tight text-foreground">{item.value}</p>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-8">
          <div className="mb-2 flex items-center justify-between">
            <p className="text-xs font-medium text-muted">
              {t.soalProgress(soalIndex + 1, soalList.length)}
            </p>
            <button type="button" onClick={() => answer(null)} className="flex items-center gap-1 text-xs font-medium text-muted hover:text-foreground">
              <SkipForward className="size-3.5" /> {t.skip}
            </button>
          </div>
          <p className="text-sm text-foreground">{t.soalPrompt}</p>
          <div className="mt-3 grid grid-cols-4 gap-2">
            {soal.displayed.map((item) => (
              <div key={item.value} className="rounded-lg border border-dashed border-border py-3.5 text-center">
                <p className="font-mono text-base font-semibold tracking-tight text-foreground">{item.value}</p>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-8">
          <p className="mb-2 text-xs font-medium text-muted">{t.chooseMissingLabel}</p>
          <div className="grid grid-cols-5 gap-2">
            {soal.referenceSet.map((item) => (
              <button
                key={item.label}
                onClick={() => answer(item.label)}
                className="flex h-14 items-center justify-center rounded-lg border border-border font-mono text-lg font-semibold text-foreground transition-colors hover:border-accent hover:bg-accent-soft hover:text-accent-soft-foreground"
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
