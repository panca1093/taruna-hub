"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card } from "@heroui/react";
import { ArrowRight, CheckCircle2, CloudOff, Lock, PlayCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { api, ApiError, type StudentTryout } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useTryoutOverviewT } from "@/lib/i18n/dict/tryout-overview";

export default function TryoutOverviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const t = useTryoutOverviewT();
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "noAccess" | "failed">("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.runtime
      .overview(id)
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
  }, [id, reloadKey]);

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
        <EmptyState
          icon={Lock}
          title={t.noAccess}
          action={
            <LinkButton href="/student/tryouts" variant="secondary" size="sm">
              {t.backToList}
            </LinkButton>
          }
        />
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
  const statusOf = (subtestId: string) => current?.subtests.find((p) => p.subtest_id === subtestId)?.status ?? "pending";
  const completedAttempts = overview.attempts.filter((a) => a.status === "completed");
  const running = overview.subtests.find((st) => statusOf(st.id) === "running");
  const upNext = current
    ? (running ?? overview.subtests.find((st) => statusOf(st.id) === "pending"))
    : overview.subtests[0];
  const totalQuestions = overview.subtests.reduce((sum, st) => sum + st.question_count, 0);
  const totalMinutes = Math.round(overview.subtests.reduce((sum, st) => sum + st.duration_seconds, 0) / 60);

  const cta = !upNext
    ? null
    : running
      ? { label: t.ctaContinueSubtest(running.name), href: `/student/tryout/${id}/subtest/${running.id}` }
      : current
        ? { label: t.ctaNextSubtest(upNext.name), href: `/student/tryout/${id}/subtest/${upNext.id}/start` }
        : {
            label: completedAttempts.length > 0 ? t.ctaRetake : t.ctaStart,
            href: `/student/tryout/${id}/subtest/${upNext.id}/start`,
          };

  const stats = [
    { label: t.statSubtests, value: overview.subtests.length },
    { label: t.statQuestions, value: totalQuestions },
    { label: t.statDuration, value: t.durationMinutes(totalMinutes) },
  ];

  return (
    <AppShell title={t.pageTitle}>
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <section className="rounded-2xl bg-accent-soft p-6 text-accent-soft-foreground">
            <div className="flex flex-wrap items-center gap-2">
              <ProgramBadge code={overview.program_code} />
              {overview.scoring_mode === "irt" && (
                <span className="rounded-md bg-surface px-2 py-0.5 text-xs font-medium text-foreground">{t.irtModeChip}</span>
              )}
            </div>
            <h1 className="font-display mt-3 text-3xl font-semibold tracking-tight text-foreground">{overview.title}</h1>
            <p className="mt-1.5 text-sm">{overview.description}</p>
            <dl className="mt-5 flex flex-wrap gap-x-8 gap-y-3">
              {stats.map((stat) => (
                <div key={stat.label}>
                  <dd className="font-display text-xl font-semibold text-foreground">{stat.value}</dd>
                  <dt className="text-xs">{stat.label}</dt>
                </div>
              ))}
            </dl>
          </section>

          <Card className="gap-0 p-6">
            <h2 className="font-display text-lg font-semibold text-foreground">{t.subtestsTitle}</h2>
            <ol className="mt-3 divide-y divide-border">
              {overview.subtests.map((st, i) => {
                const status = statusOf(st.id);
                const isNext = upNext?.id === st.id;
                const locked = status === "pending" && !isNext;
                return (
                  <li key={st.id} className={cn("flex items-center gap-4 py-4", locked && "opacity-60")}>
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface-secondary font-mono text-sm text-muted">
                      {i + 1}
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-foreground">{st.name}</p>
                      <p className="text-xs text-muted">{t.subtestMeta(Math.round(st.duration_seconds / 60), st.question_count)}</p>
                    </div>
                    {status === "submitted" ? (
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-success">
                        <CheckCircle2 className="size-3.5" /> {t.completedStatus}
                      </span>
                    ) : status === "running" ? (
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-accent">
                        <PlayCircle className="size-3.5" /> {t.runningStatus}
                      </span>
                    ) : locked ? (
                      <span className="inline-flex items-center gap-1 text-xs text-muted">
                        <Lock className="size-3" /> {t.lockedStatus}
                      </span>
                    ) : (
                      <span className="text-xs text-muted">{t.pendingStatus}</span>
                    )}
                  </li>
                );
              })}
            </ol>
            <p className="mt-2 rounded-lg border border-border bg-surface-secondary p-3 text-xs text-muted">{t.rulesNotice}</p>
          </Card>

          <Card className="gap-0 p-6">
            <h2 className="font-display text-lg font-semibold text-foreground">{t.historyTitle}</h2>
            {completedAttempts.length === 0 ? (
              <p className="mt-3 text-sm text-muted">{t.historyEmpty}</p>
            ) : (
              <ul className="mt-3 divide-y divide-border">
                {completedAttempts.map((attempt) => (
                  <li key={attempt.session_id}>
                    <Link
                      href={`/student/tryout/${id}/result?attempt=${attempt.session_id}`}
                      aria-label={t.openAttemptLabel(attempt.attempt_number)}
                      className="flex items-center justify-between gap-3 rounded-lg py-4 transition-colors hover:bg-surface-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                    >
                      <span>
                        <span className="block text-sm font-medium text-foreground">{t.attemptLabel(attempt.attempt_number)}</span>
                        {attempt.completed_at && (
                          <span className="block text-xs text-muted">{new Date(attempt.completed_at).toLocaleString()}</span>
                        )}
                      </span>
                      <span className="flex items-center gap-3">
                        <span className="font-display text-2xl font-semibold text-foreground">{attempt.score ?? "–"}</span>
                        <ArrowRight className="size-4 text-muted" />
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>

        <aside className="lg:col-span-1">
          <Card className="gap-0 p-6 lg:sticky lg:top-20">
            <p className="text-sm text-muted">{t.accessTitle}</p>
            <p className="font-display mt-1 text-2xl font-semibold text-foreground">{t.accessOwned}</p>
            <p className="mt-1.5 text-sm text-muted">{t.accessUntil(new Date(overview.active_until).toLocaleDateString())}</p>
            {current && <p className="mt-3 text-xs text-muted">{t.inProgressNote}</p>}
            <div className="mt-5 flex flex-col gap-2">
              {cta && (
                <LinkButton href={cta.href} variant="primary" size="md" className="w-full">
                  {cta.label} <ArrowRight className="size-4" />
                </LinkButton>
              )}
              <LinkButton href="/student/tryouts" variant="ghost" size="md" className="w-full">
                {t.ctaBack}
              </LinkButton>
            </div>
          </Card>
        </aside>
      </div>
    </AppShell>
  );
}
