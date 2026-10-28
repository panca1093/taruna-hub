"use client";

import { use, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { CheckCircle2, ChevronRight, Circle, Clock, CloudOff, Lock, PlayCircle, RotateCcw, Trophy } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { api, ApiError, errorText, type StudentTryout } from "@/lib/api/client";
import { useTryoutOverviewT } from "@/lib/i18n/dict/tryout-overview";

export default function TryoutOverviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const t = useTryoutOverviewT();
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "noAccess" | "failed">("loading");
  const [busy, setBusy] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const load = useCallback(() => api.runtime.overview(id), [id]);

  useEffect(() => {
    let cancelled = false;
    load()
      .then((loaded) => {
        if (cancelled) return;
        setOverview(loaded);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (!cancelled) setState(err instanceof ApiError && err.status === 404 ? "noAccess" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [load, reloadKey]);

  if (state === "loading") {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      </AppShell>
    );
  }
  if (state === "noAccess") {
    return (
      <AppShell title={t.pageTitle}>
        <p className="text-sm text-muted">{t.noAccess}</p>
      </AppShell>
    );
  }
  if (state === "failed" || !overview) {
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
                setState("loading");
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

  const current = overview.current;
  const partOf = (subtestId: string) => current?.subtests.find((p) => p.subtest_id === subtestId);
  const completedAttempts = overview.attempts.filter((a) => a.status === "completed");
  const upNext = current ? overview.subtests.find((st) => partOf(st.id)?.status !== "submitted") : overview.subtests[0];

  const fail = (err: unknown) => toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });

  // Starting a subtest is the student's explicit action: the attempt exists first, then this part's clock starts.
  const handleStart = async (subtestId: string) => {
    setBusy(true);
    try {
      const session = current ?? (await api.runtime.start(id));
      await api.runtime.startSubtest(session.id, subtestId);
      router.push(`/student/tryout/${id}/subtest/${subtestId}`);
    } catch (err) {
      fail(err);
      setReloadKey((key) => key + 1);
    } finally {
      setBusy(false);
    }
  };

  const handleRetake = async () => {
    setBusy(true);
    try {
      await api.runtime.start(id);
      setReloadKey((key) => key + 1);
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <AppShell title={t.pageTitle}>
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center gap-2">
          <ProgramBadge code={overview.program_code} />
          {overview.scoring_mode === "irt" && (
            <span className="rounded-md bg-accent-soft px-2 py-0.5 text-xs font-medium text-accent-soft-foreground">
              {t.irtModeChip}
            </span>
          )}
        </div>
        <h1 className="font-display mt-2 text-2xl font-semibold tracking-tight text-foreground">{overview.title}</h1>
        <p className="mt-1.5 text-sm text-muted">{overview.description}</p>

        {!current && completedAttempts.length > 0 && (
          <Card className="mt-6 flex-row items-center justify-between gap-3 border-success/30 bg-success-soft p-4">
            <div className="flex items-center gap-2.5">
              <Trophy className="size-5 text-success-soft-foreground" />
              <p className="text-sm font-medium text-success-soft-foreground">{t.completedBanner}</p>
            </div>
            <div className="flex shrink-0 gap-2">
              <Button variant="secondary" size="sm" isDisabled={busy} onPress={() => void handleRetake()}>
                <RotateCcw className="size-3.5" /> {t.retakeButton}
              </Button>
              <LinkButton href={`/student/tryout/${id}/result`} variant="primary" size="sm">
                {t.viewResultButton}
              </LinkButton>
            </div>
          </Card>
        )}

        {completedAttempts.length > 0 && (
          <div className="mt-6">
            <p className="mb-2 text-sm font-semibold text-foreground">{t.attemptHistoryTitle}</p>
            <div className="space-y-2">
              {completedAttempts.map((attempt) => (
                <Card key={attempt.session_id} className="flex-row items-center justify-between gap-3 p-3.5">
                  <span className="text-sm font-medium text-foreground">{t.attemptLabel(attempt.attempt_number)}</span>
                  <span className="flex items-center gap-3 text-xs text-muted">
                    {t.scoreLabel} <span className="font-mono font-semibold text-foreground">{attempt.score}</span>
                    {attempt.completed_at && new Date(attempt.completed_at).toLocaleString()}
                  </span>
                </Card>
              ))}
            </div>
          </div>
        )}

        <div className="mt-6 space-y-3">
          <p className="text-sm font-semibold text-foreground">{t.subtestsTitle}</p>
          {overview.subtests.map((st, i) => {
            const status = partOf(st.id)?.status ?? "pending";
            const canStart = upNext?.id === st.id && (current !== null || completedAttempts.length === 0);
            const locked = status === "pending" && !canStart;

            return (
              <Card key={st.id} className={`flex-row items-center gap-4 p-4 ${locked ? "opacity-60" : ""}`}>
                <span
                  className={`flex size-9 shrink-0 items-center justify-center rounded-full ${
                    status === "submitted"
                      ? "bg-success-soft text-success-soft-foreground"
                      : canStart
                        ? "bg-gradient-ring text-white"
                        : "bg-surface-secondary text-muted"
                  }`}
                >
                  {status === "submitted" ? (
                    <CheckCircle2 className="size-4" />
                  ) : locked ? (
                    <Lock className="size-3.5" />
                  ) : canStart ? (
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
                    {t.subtestMeta(Math.round(st.duration_seconds / 60), st.question_count)}
                  </p>
                </div>
                {status === "submitted" ? (
                  <span className="text-xs font-medium text-success">{t.completedStatus}</span>
                ) : status === "running" ? (
                  <LinkButton href={`/student/tryout/${id}/subtest/${st.id}`} variant="primary" size="sm">
                    {t.continueButton} <ChevronRight className="size-3.5" />
                  </LinkButton>
                ) : canStart ? (
                  <Button variant="primary" size="sm" isDisabled={busy} onPress={() => void handleStart(st.id)}>
                    {t.startButton} <ChevronRight className="size-3.5" />
                  </Button>
                ) : (
                  <span className="text-xs text-muted">{t.lockedStatus}</span>
                )}
              </Card>
            );
          })}
        </div>

        <p className="mt-6 rounded-lg border border-dashed border-border p-3 text-xs text-muted">{t.footerNotice}</p>
      </div>
    </AppShell>
  );
}
