"use client";

import { useEffect, useState } from "react";
import { Button, Card, Chip } from "@heroui/react";
import { Clock, CloudOff, ListChecks, PackageOpen, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { useTryoutsListT } from "@/lib/i18n/dict/tryouts-list";
import { api, type AttemptSummary, type Enrollment, type KecermatanStudentTest } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useHydrated } from "@/lib/use-hydrated";

type TryoutState = "not_started" | "in_progress" | "completed";
type Tab = "tryout" | "kecermatan";

const completedTime = (a: AttemptSummary) => (a.completed_at ? new Date(a.completed_at).getTime() : 0);

export default function StudentTryoutsPage() {
  const t = useTryoutsListT();
  const hydrated = useHydrated();

  const [enrollments, setEnrollments] = useState<Enrollment[] | null>(null);
  const [attempts, setAttempts] = useState<AttemptSummary[]>([]);
  const [kecTests, setKecTests] = useState<KecermatanStudentTest[]>([]);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [pickedTab, setPickedTab] = useState<Tab | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.orders.enrollments(), api.runtime.attempts(), api.kecermatanRuntime.tests()])
      .then(([list, history, kecermatan]) => {
        if (cancelled) return;
        setEnrollments(list.enrollments);
        setAttempts(history.attempts);
        setKecTests(kecermatan.tests);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const held = enrollments ?? [];
  const shellProps = { title: t.pageTitle, description: t.pageDescription } as const;

  if (loadFailed || enrollments === null) {
    return (
      <AppShell {...shellProps}>
        {loadFailed ? (
          <EmptyState
            icon={CloudOff}
            title={t.loadErrorTitle}
            description={t.loadErrorDescription}
            action={
              <Button
                variant="primary"
                size="sm"
                onPress={() => {
                  setLoadFailed(false);
                  setReloadKey((key) => key + 1);
                }}
              >
                {t.retry}
              </Button>
            }
          />
        ) : (
          <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
        )}
      </AppShell>
    );
  }

  if (held.length === 0 && kecTests.length === 0) {
    return (
      <AppShell {...shellProps}>
        <EmptyState
          icon={PackageOpen}
          title={t.emptyTitle}
          description={t.emptyDescription}
          action={
            <LinkButton href="/catalog" variant="primary" size="sm">
              {t.emptyCta}
            </LinkButton>
          }
        />
      </AppShell>
    );
  }

  const tab: Tab = pickedTab ?? (held.length === 0 ? "kecermatan" : "tryout");
  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: "tryout", label: t.tabTryout, count: held.length },
    { id: "kecermatan", label: t.tabKecermatan, count: kecTests.length },
  ];

  return (
    <AppShell {...shellProps}>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.heading}</h2>
          <p className="mt-1 text-sm text-muted">{t.headingDescription}</p>
        </div>
        <LinkButton href="/catalog" variant="secondary" size="sm">
          {t.browseCatalog}
        </LinkButton>
      </div>

      <div role="tablist" aria-label={t.tabsLabel} className="mt-6 flex gap-2 border-b border-border">
        {tabs.map((item) => {
          const active = tab === item.id;
          return (
            <button
              key={item.id}
              type="button"
              role="tab"
              id={`practice-tab-${item.id}`}
              aria-selected={active}
              aria-controls={`practice-panel-${item.id}`}
              onClick={() => setPickedTab(item.id)}
              className={cn(
                "-mb-px inline-flex items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus",
                active ? "border-accent text-foreground" : "border-transparent text-muted hover:text-foreground",
              )}
            >
              {item.label}
              <span className="rounded-full bg-surface-secondary px-2 py-0.5 text-xs text-muted">{item.count}</span>
            </button>
          );
        })}
      </div>

      <div role="tabpanel" id={`practice-panel-${tab}`} aria-labelledby={`practice-tab-${tab}`} className="mt-6">
        {tab === "tryout" ? (
          held.length === 0 ? (
            <TabEmpty title={t.emptyTryoutTabTitle} description={t.emptyTabDescription} cta={t.emptyCta} />
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {held.map((e) => {
                const own = hydrated ? attempts.filter((a) => a.tryout_id === e.tryout_id) : [];
                const done = own.filter((a) => a.status === "completed").sort((a, b) => completedTime(b) - completedTime(a));
                const latest = done[0];
                const state: TryoutState = !hydrated
                  ? "not_started"
                  : own.some((a) => a.status === "in_progress")
                    ? "in_progress"
                    : done.length > 0
                      ? "completed"
                      : "not_started";

                return (
                  <Card key={e.id} className="gap-0 overflow-hidden p-0">
                    <div className="h-20 w-full bg-gradient-ring" />
                    <div className="space-y-3 p-4">
                      <div className="flex items-center justify-between gap-2">
                        <ProgramBadge code={e.program_code} />
                        <Chip
                          size="sm"
                          className={`font-medium ${
                            state === "completed"
                              ? "bg-success-soft text-success-soft-foreground"
                              : state === "in_progress"
                                ? "bg-accent-soft text-accent-soft-foreground"
                                : "bg-default text-default-foreground"
                          }`}
                        >
                          {state === "completed" ? t.stateCompleted : state === "in_progress" ? t.stateInProgress : t.stateNotStarted}
                        </Chip>
                      </div>
                      <p className="line-clamp-1 text-sm font-semibold text-foreground">{e.tryout_title}</p>
                      <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted">
                        <span className="inline-flex items-center gap-1">
                          <ListChecks className="size-3.5" /> {t.subtestCount(e.subtest_count)}
                        </span>
                        <span className="inline-flex items-center gap-1">
                          <Clock className="size-3.5" /> {t.activeUntil(new Date(e.active_until).toLocaleDateString())}
                        </span>
                      </div>
                      {latest && (
                        <p className="text-xs text-muted">
                          {t.attemptsDone(done.length)}
                          {latest.score !== null && ` · ${t.latestScore(latest.score)}`}
                        </p>
                      )}
                      {state === "completed" && latest ? (
                        <div className="flex gap-2">
                          <LinkButton
                            href={`/student/tryout/${e.tryout_id}/result?attempt=${latest.session_id}`}
                            variant="secondary"
                            size="sm"
                            className="flex-1"
                          >
                            {t.ctaViewResult}
                          </LinkButton>
                          <LinkButton href={`/student/tryout/${e.tryout_id}`} variant="ghost" size="sm" className="flex-1">
                            {t.ctaHistory}
                          </LinkButton>
                        </div>
                      ) : (
                        <LinkButton href={`/student/tryout/${e.tryout_id}`} variant="primary" size="sm" className="w-full">
                          {state === "in_progress" ? t.ctaContinue : t.ctaStart}
                        </LinkButton>
                      )}
                    </div>
                  </Card>
                );
              })}
            </div>
          )
        ) : kecTests.length === 0 ? (
          <TabEmpty title={t.emptyKecermatanTabTitle} description={t.emptyTabDescription} cta={t.emptyCta} />
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {kecTests.map((test) => {
              const left = test.attempts_remaining;

              return (
                <Card key={test.test_id} className="gap-0 overflow-hidden p-0">
                  <div className="h-20 w-full" style={{ background: `linear-gradient(135deg, ${test.cover_from}, ${test.cover_to})` }} />
                  <div className="space-y-3 p-4">
                    <div className="flex items-center justify-between gap-2">
                      <ProgramBadge code={test.program_code} />
                      <Chip size="sm" className={`font-medium ${left > 0 ? "bg-accent-soft text-accent-soft-foreground" : "bg-danger-soft text-danger-soft-foreground"}`}>
                        {left > 0 ? t.attemptsRemaining(left) : t.quotaExhausted}
                      </Chip>
                    </div>
                    <p className="line-clamp-1 text-sm font-semibold text-foreground">{test.title}</p>
                    <p className="flex items-center gap-1.5 text-xs text-muted">
                      <Sparkles className="size-3.5" /> {t.kolomSoalStat(test.kolom_count, test.soal_per_kolom)}
                    </p>
                    <LinkButton href={`/student/kecermatan/${test.test_id}`} variant={left > 0 ? "primary" : "secondary"} size="sm" className="w-full">
                      {left > 0 ? t.ctaWork : t.ctaViewHistory}
                    </LinkButton>
                  </div>
                </Card>
              );
            })}
          </div>
        )}
      </div>
    </AppShell>
  );
}

function TabEmpty({ title, description, cta }: { title: string; description: string; cta: string }) {
  return (
    <EmptyState
      icon={PackageOpen}
      title={title}
      description={description}
      action={
        <LinkButton href="/catalog" variant="primary" size="sm">
          {cta}
        </LinkButton>
      }
    />
  );
}
