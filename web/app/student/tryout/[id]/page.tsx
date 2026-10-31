"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@heroui/react";
import { ArrowRight, CheckCircle2, CloudOff, Lock, PlayCircle, ShieldCheck } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { Icon } from "@/components/icon";
import { NoteBox, Panel, CardKicker, TextLink } from "@/components/page-blocks";
import { Breadcrumb } from "@/components/page-heading";
import { StatusChip } from "@/components/status-badge";
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
        <p className="py-14 text-center text-small text-muted">{t.loading}</p>
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
    <AppShell>
      <Breadcrumb items={[{ label: t.crumbDashboard, href: "/student" }, { label: t.crumbTryout }]} />
      <div className="grid gap-6 lg:grid-cols-3 lg:gap-7">
        <div className="space-y-6 lg:col-span-2">
          <Panel as="section" tone="peach" radius="hero">
            <CardKicker icon={ShieldCheck} className="mb-4">
              {overview.program_code}
              {overview.scoring_mode === "irt" && (
                <StatusChip tone="neutral" className="ml-2 bg-surface">
                  {t.irtModeChip}
                </StatusChip>
              )}
            </CardKicker>
            <h1 className="font-display max-w-2xl text-hero-lg sm:text-display-sm">{overview.title}</h1>
            <p className="mt-3 max-w-prose text-small text-muted">{overview.description}</p>
            <dl className="mt-6 flex flex-wrap gap-x-8 gap-y-4 sm:gap-x-8">
              {stats.map((stat) => (
                <div key={stat.label}>
                  <dd className="font-display text-heading">{stat.value}</dd>
                  <dt className="mt-0.5 text-micro text-muted">{stat.label}</dt>
                </div>
              ))}
            </dl>
          </Panel>

          <Panel as="section">
            <h2 className="font-display text-heading-sm">{t.subtestsTitle}</h2>
            <ol className="mt-3 divide-y divide-border">
              {overview.subtests.map((st, i) => {
                const status = statusOf(st.id);
                const isNext = upNext?.id === st.id;
                const locked = status === "pending" && !isNext;
                return (
                  <li key={st.id} className={cn("flex items-center gap-4 py-4", locked && "opacity-60")}>
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-control bg-background text-label text-muted">
                      {i + 1}
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-caption font-medium">{st.name}</p>
                      <p className="mt-1 text-micro text-muted">{t.subtestMeta(Math.round(st.duration_seconds / 60), st.question_count)}</p>
                    </div>
                    {status === "submitted" ? (
                      <span className="inline-flex items-center gap-1.5 text-label font-medium text-success">
                        <Icon icon={CheckCircle2} size="xs" /> {t.completedStatus}
                      </span>
                    ) : status === "running" ? (
                      <span className="inline-flex items-center gap-1.5 text-label font-medium text-accent">
                        <Icon icon={PlayCircle} size="xs" /> {t.runningStatus}
                      </span>
                    ) : locked ? (
                      <span className="inline-flex items-center gap-1.5 text-label text-muted">
                        <Icon icon={Lock} size="xs" /> {t.lockedStatus}
                      </span>
                    ) : (
                      <span className="text-label text-muted">{t.pendingStatus}</span>
                    )}
                  </li>
                );
              })}
            </ol>
            <NoteBox className="mt-5">{t.rulesNotice}</NoteBox>
          </Panel>

          <Panel as="section">
            <h2 className="font-display text-heading-sm">{t.historyTitle}</h2>
            {completedAttempts.length === 0 ? (
              <p className="mt-3 text-caption leading-relaxed text-muted">{t.historyEmpty}</p>
            ) : (
              <ul className="mt-2 divide-y divide-border">
                {completedAttempts.map((attempt) => (
                  <li key={attempt.session_id}>
                    <Link
                      href={`/student/tryout/${id}/result?attempt=${attempt.session_id}`}
                      aria-label={t.openAttemptLabel(attempt.attempt_number)}
                      className="flex items-center justify-between gap-4 rounded-control py-4 text-caption outline-none transition-colors hover:text-accent focus-visible:ring-2 focus-visible:ring-focus"
                    >
                      <span>
                        <span className="block">{t.attemptLabel(attempt.attempt_number)}</span>
                        {attempt.completed_at && (
                          <span className="mt-1 block text-micro text-muted">{new Date(attempt.completed_at).toLocaleString()}</span>
                        )}
                      </span>
                      <span className="flex items-center gap-5">
                        <span className="font-display text-heading">{attempt.score ?? "–"}</span>
                        <Icon icon={ArrowRight} size="sm" />
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>

        <aside className="lg:col-span-1">
          <Panel as="div" className="lg:sticky lg:top-6">
            <p className="mb-3 text-label text-muted">{t.accessTitle}</p>
            <p className="font-display text-display-sm">{t.accessOwned}</p>
            <p className="mt-1 mb-5 text-label text-muted">{t.accessUntil(new Date(overview.active_until).toLocaleDateString())}</p>
            <div className="flex flex-col gap-2">
              {cta && (
                <LinkButton href={cta.href} variant="primary" size="md" className="w-full">
                  {cta.label} <Icon icon={ArrowRight} size="sm" />
                </LinkButton>
              )}
              <TextLink href="/student/tryouts" className="self-start">
                {t.ctaBack}
              </TextLink>
            </div>
            {current && <NoteBox className="mt-5">{t.inProgressNote}</NoteBox>}
          </Panel>
        </aside>
      </div>
    </AppShell>
  );
}
