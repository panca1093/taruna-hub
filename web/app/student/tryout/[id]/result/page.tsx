"use client";

import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { Clock, CloudOff, Trophy } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { api, ApiError, type StudentTryout, type TryoutResult } from "@/lib/api/client";
import { useTryoutResultT } from "@/lib/i18n/dict/tryout-result";

export default function ResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const t = useTryoutResultT();
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [result, setResult] = useState<TryoutResult | null>(null);
  const [failed, setFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.runtime
      .overview(id)
      .then(async (loaded) => {
        const latest = loaded.attempts.find((a) => a.status === "completed");
        if (!latest) {
          if (!cancelled) router.replace(`/student/tryout/${id}`);
          return;
        }
        const res = await api.runtime.result(latest.session_id);
        if (cancelled) return;
        setOverview(loaded);
        setResult(res);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) router.replace(`/student/tryout/${id}`);
        else setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [id, router, reloadKey]);

  if (failed) {
    return (
      <AppShell title={t.pageTitle}>
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
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
          }
        />
      </AppShell>
    );
  }
  if (!result || !overview) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  return (
    <AppShell title={t.pageTitle} description={overview.title}>
      <div className="mx-auto max-w-3xl space-y-6">
        <Card className="flex-col items-center gap-4 p-8 text-center sm:flex-row sm:text-left">
          <CountdownRing value={result.score} size={110} strokeWidth={9} animate>
            <div className="text-center">
              <p className="font-display text-2xl font-semibold text-foreground">{result.score}</p>
              <p className="text-[10px] text-muted">/ 100</p>
            </div>
          </CountdownRing>
          <div className="flex-1">
            <p className="text-sm font-medium text-muted">{t.yourTotalScore}</p>
            <p className="font-display mt-0.5 text-xl font-semibold text-foreground">{overview.title}</p>
            <div className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent-soft-foreground">
              <Trophy className="size-3.5" />
              {t.rankPending}
            </div>
          </div>
        </Card>

        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.subtestScoresTitle}</p>
          <div className="grid gap-3 sm:grid-cols-3">
            {result.subtests.map((s) => (
              <Card key={s.subtest_id} className="gap-2 p-4">
                <p className="line-clamp-1 text-xs font-medium text-muted">{s.name}</p>
                <p className="font-display text-xl font-semibold text-foreground">{s.score}</p>
                <p className="text-xs text-muted">{t.correctCount(s.correct_count, s.question_count)}</p>
              </Card>
            ))}
          </div>
        </div>

        <div className="flex items-center justify-between">
          <p className="text-sm font-semibold text-foreground">{t.attemptHistoryTitle}</p>
          <LinkButton href={`/student/tryout/${id}`} variant="ghost" size="sm">
            {t.viewAllAttempts}
          </LinkButton>
        </div>

        <div className="flex items-start gap-3 rounded-xl border border-border bg-surface-secondary p-4 text-sm text-muted">
          <Clock className="mt-0.5 size-4 shrink-0" />
          <p>{t.explanationsPending}</p>
        </div>
      </div>
    </AppShell>
  );
}
