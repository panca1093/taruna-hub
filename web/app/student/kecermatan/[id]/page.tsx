"use client";

import { use, useEffect, useState } from "react";
import { notFound, useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { Gauge, PlayCircle, ShoppingBag, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { useSession } from "@/lib/session-context";
import {
  computeResult,
  getAttempts,
  initSession,
  loadSession,
  remainingAttempts,
  startNewAttempt,
  type KecermatanSessionState,
} from "@/lib/kecermatan-session";
import {
  formatDateTime,
  getKecermatanAttemptsPurchased,
  getKecermatanTest,
  getProductForKecermatanTest,
  getProgram,
  hasKecermatanAccess,
} from "@/lib/mock-data";
import { useKecermatanOverviewT } from "@/lib/i18n/dict/kecermatan-overview";

export default function KecermatanOverviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const { user } = useSession();
  const t = useKecermatanOverviewT();
  const test = getKecermatanTest(id);
  const [session, setSession] = useState<KecermatanSessionState | null>(null);
  const [left, setLeft] = useState(0);
  const [attempts, setAttempts] = useState<KecermatanSessionState[]>([]);

  useEffect(() => {
    if (!test) return;
    setSession(loadSession(test.id, user.id));
    const purchased = getKecermatanAttemptsPurchased(user.id, test.id);
    setLeft(remainingAttempts(test.id, user.id, purchased));
    setAttempts(getAttempts(test.id, user.id));
  }, [test, user.id]);

  if (!test) notFound();
  if (!hasKecermatanAccess(user.id, test.id)) {
    return (
      <AppShell title={test.title}>
        <p className="text-sm text-muted">{t.noAccess}</p>
      </AppShell>
    );
  }

  const program = getProgram(test.programId);
  const product = getProductForKecermatanTest(test.id);
  const inProgress = session?.status === "in_progress" && Object.keys(session.kolomStartedAt).length > 0;

  const handleStart = () => {
    startNewAttempt(test.id, user.id);
    router.push(`/student/kecermatan/${test.id}/attempt`);
  };

  const handleContinue = () => {
    initSession(test.id, user.id);
    router.push(`/student/kecermatan/${test.id}/attempt`);
  };

  return (
    <AppShell title={t.pageTitle}>
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center gap-2">
          {program && <ProgramBadge code={program.code} />}
          <span className="rounded-md bg-accent-soft px-2 py-0.5 text-xs font-medium text-accent-soft-foreground">
            {t.chipKecermatan}
          </span>
        </div>
        <h1 className="font-display mt-2 text-2xl font-semibold tracking-tight text-foreground">{test.title}</h1>
        <p className="mt-1.5 text-sm text-muted">{test.description}</p>

        <Card className="mt-6 flex-row items-center justify-between gap-3 p-4">
          <div className="flex items-center gap-2.5">
            <Sparkles className="size-5 text-accent" />
            <div>
              <p className="text-sm font-medium text-foreground">{t.attemptsRemaining(left)}</p>
              <p className="text-xs text-muted">
                {t.kolomStat(test.kolomCount, test.soalPerKolom, Math.round(test.kolomDurationSeconds / 60))}
              </p>
            </div>
          </div>
          {inProgress ? (
            <Button variant="primary" size="sm" onPress={handleContinue}>
              <PlayCircle className="size-3.5" /> {t.ctaContinue}
            </Button>
          ) : left > 0 ? (
            <Button variant="primary" size="sm" onPress={handleStart}>
              <PlayCircle className="size-3.5" /> {t.ctaStartNew}
            </Button>
          ) : product ? (
            <LinkButton href={`/catalog/${product.id}`} variant="secondary" size="sm">
              <ShoppingBag className="size-3.5" /> {t.ctaBuyMore}
            </LinkButton>
          ) : null}
        </Card>

        {attempts.length > 0 && (
          <div className="mt-6">
            <p className="mb-2 text-sm font-semibold text-foreground">{t.historyTitle}</p>
            <div className="space-y-2">
              {attempts.map((attempt) => {
                const result = computeResult(test, attempt);
                return (
                  <Card key={attempt.attemptNumber} className="flex-row items-center justify-between gap-3 p-3.5">
                    <span className="text-sm font-medium text-foreground">{t.attemptLabel(attempt.attemptNumber)}</span>
                    <span className="flex items-center gap-3 text-xs text-muted">
                      {t.nilaiAkhir} <span className="font-mono font-semibold text-foreground">{result.nilaiAkhirScore}</span>
                      {attempt.completedAt && formatDateTime(attempt.completedAt)}
                    </span>
                  </Card>
                );
              })}
            </div>
          </div>
        )}

        {attempts.length === 0 && (
          <div className="mt-6 flex items-start gap-3 rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            <Gauge className="mt-0.5 size-4 shrink-0" />
            <p>{t.noHistoryInfo}</p>
          </div>
        )}
      </div>
    </AppShell>
  );
}
