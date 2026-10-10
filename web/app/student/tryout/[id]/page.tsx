"use client";

import { use, useState } from "react";
import { notFound, useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { CheckCircle2, ChevronRight, Circle, Clock, Lock, PlayCircle, RotateCcw, Trophy } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { LinkButton } from "@/components/ui/link-button";
import { useSession } from "@/lib/session-context";
import { computeResult, getAttempts, initSession, nextPendingSubtest, startNewAttempt, type TryoutSessionState } from "@/lib/tryout-session";
import { formatDateTime, getProgram, getSubtestsForTryout, getTryout, hasEnrollment } from "@/lib/mock-data";
import { ProgramBadge } from "@/components/status-badge";
import { useHydrated } from "@/lib/use-hydrated";
import { useTryoutOverviewT } from "@/lib/i18n/dict/tryout-overview";

export default function TryoutOverviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const { user } = useSession();
  const tryout = getTryout(id);
  const [session, setSession] = useState<TryoutSessionState | null>(null);
  const t = useTryoutOverviewT();

  const hydrated = useHydrated();

  if (hydrated && tryout && (session?.tryoutId !== tryout.id || session.userId !== user.id)) {
    setSession(initSession(tryout.id, user.id));
  }

  if (!tryout) notFound();
  if (!hasEnrollment(user.id, tryout.id)) {
    return (
      <AppShell title={tryout.title}>
        <p className="text-sm text-muted">{t.noAccess}</p>
      </AppShell>
    );
  }

  const subtests = getSubtestsForTryout(tryout.id);
  const program = getProgram(tryout.programId);
  const upNext = session ? nextPendingSubtest(tryout.id, session) : subtests[0];
  const attempts = getAttempts(tryout.id, user.id);

  const handleRetake = () => {
    startNewAttempt(tryout.id, user.id);
    setSession(initSession(tryout.id, user.id));
    if (subtests[0]) router.push(`/student/tryout/${tryout.id}/subtest/${subtests[0].id}`);
  };

  return (
    <AppShell title={t.pageTitle}>
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center gap-2">
          {program && <ProgramBadge code={program.code} />}
          {tryout.scoringMode === "irt" && (
            <span className="rounded-md bg-accent-soft px-2 py-0.5 text-xs font-medium text-accent-soft-foreground">
              {t.irtModeChip}
            </span>
          )}
        </div>
        <h1 className="font-display mt-2 text-2xl font-semibold tracking-tight text-foreground">{tryout.title}</h1>
        <p className="mt-1.5 text-sm text-muted">{tryout.description}</p>

        {session?.status === "completed" && (
          <Card className="mt-6 flex-row items-center justify-between gap-3 border-success/30 bg-success-soft p-4">
            <div className="flex items-center gap-2.5">
              <Trophy className="size-5 text-success-soft-foreground" />
              <p className="text-sm font-medium text-success-soft-foreground">
                {t.completedBanner}
              </p>
            </div>
            <div className="flex shrink-0 gap-2">
              <Button variant="secondary" size="sm" onPress={handleRetake}>
                <RotateCcw className="size-3.5" /> {t.retakeButton}
              </Button>
              <LinkButton href={`/student/tryout/${tryout.id}/result`} variant="primary" size="sm">
                {t.viewResultButton}
              </LinkButton>
            </div>
          </Card>
        )}

        {attempts.length > 0 && (
          <div className="mt-6">
            <p className="mb-2 text-sm font-semibold text-foreground">{t.attemptHistoryTitle}</p>
            <div className="space-y-2">
              {attempts.map((attempt) => {
                const result = computeResult(tryout, attempt, { rank: 1, participantCount: 1 });
                return (
                  <Card key={attempt.attemptNumber} className="flex-row items-center justify-between gap-3 p-3.5">
                    <span className="text-sm font-medium text-foreground">{t.attemptLabel(attempt.attemptNumber)}</span>
                    <span className="flex items-center gap-3 text-xs text-muted">
                      {t.scoreLabel} <span className="font-mono font-semibold text-foreground">{result.totalScore}</span>
                      {attempt.completedAt && formatDateTime(attempt.completedAt)}
                    </span>
                  </Card>
                );
              })}
            </div>
          </div>
        )}

        <div className="mt-6 space-y-3">
          <p className="text-sm font-semibold text-foreground">{t.subtestsTitle}</p>
          {subtests.map((st, i) => {
            const status = session?.subtestStatus[st.id] ?? "pending";
            const isNext = upNext?.id === st.id && session?.status !== "completed";
            const locked = status === "pending" && !isNext;

            return (
              <Card
                key={st.id}
                className={`flex-row items-center gap-4 p-4 ${locked ? "opacity-60" : ""}`}
              >
                <span
                  className={`flex size-9 shrink-0 items-center justify-center rounded-full ${
                    status === "completed"
                      ? "bg-success-soft text-success-soft-foreground"
                      : isNext
                        ? "bg-gradient-ring text-white"
                        : "bg-surface-secondary text-muted"
                  }`}
                >
                  {status === "completed" ? (
                    <CheckCircle2 className="size-4" />
                  ) : locked ? (
                    <Lock className="size-3.5" />
                  ) : isNext ? (
                    <PlayCircle className="size-4" />
                  ) : (
                    <Circle className="size-4" />
                  )}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-foreground">
                    {i + 1}. {st.name}
                  </p>
                  <p className="flex items-center gap-1.5 text-xs text-muted">
                    <Clock className="size-3 shrink-0" />
                    {t.subtestMeta(Math.round(st.durationSeconds / 60), st.questionIds.length)}
                  </p>
                </div>
                {status === "completed" ? (
                  <span className="text-xs font-medium text-success">{t.completedStatus}</span>
                ) : isNext ? (
                  <LinkButton href={`/student/tryout/${tryout.id}/subtest/${st.id}`} variant="primary" size="sm">
                    {status === "in_progress" ? t.continueButton : t.startButton} <ChevronRight className="size-3.5" />
                  </LinkButton>
                ) : (
                  <span className="text-xs text-muted">{t.lockedStatus}</span>
                )}
              </Card>
            );
          })}
        </div>

        <p className="mt-6 rounded-lg border border-dashed border-border p-3 text-xs text-muted">
          {t.footerNotice}
        </p>
      </div>
    </AppShell>
  );
}
