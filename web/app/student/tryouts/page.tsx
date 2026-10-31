"use client";

import { useEffect, useState } from "react";
import { Button, Input } from "@heroui/react";
import { ArrowRight, BookOpen, Check, CloudOff, Clock, ListChecks, PackageOpen, Search, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { Icon } from "@/components/icon";
import { UnderlineTabs } from "@/components/filter-pills";
import { TextLink } from "@/components/page-blocks";
import { ProgramBadge, StatusChip } from "@/components/status-badge";
import { useTryoutsListT } from "@/lib/i18n/dict/tryouts-list";
import { api, type AttemptSummary, type Enrollment, type KecermatanStudentTest } from "@/lib/api/client";
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
  const [search, setSearch] = useState("");

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
  const catalogLink = (
    <LinkButton href="/catalog" variant="outline" size="sm">
      {t.browseCatalog}
      <Icon icon={ArrowRight} size="xs" />
    </LinkButton>
  );
  const shellProps = { title: t.heading, description: t.headingDescription, actions: catalogLink } as const;

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
          <p className="py-14 text-center text-small text-muted">{t.loading}</p>
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
  const query = search.trim().toLowerCase();
  const shownTryouts = held.filter((e) => e.tryout_title.toLowerCase().includes(query));
  const shownTests = kecTests.filter((test) => test.title.toLowerCase().includes(query));
  const gridClass = "grid gap-5 sm:grid-cols-2 xl:grid-cols-3";

  return (
    <AppShell {...shellProps}>
      <UnderlineTabs
        options={tabs}
        value={tab}
        onChange={(id) => {
          setPickedTab(id as Tab);
          setSearch("");
        }}
        ariaLabel={t.tabsLabel}
      />

      <div role="tabpanel" aria-label={tabs.find((item) => item.id === tab)?.label}>
        {(tab === "tryout" ? held.length > 0 : kecTests.length > 0) && (
          <div className="mb-6 flex">
            <div className="relative w-full sm:ml-auto sm:w-56">
              <Icon icon={Search} size="xs" className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
              <Input fullWidth aria-label={t.searchLabel} placeholder={t.searchPlaceholder} value={search} onChange={(e) => setSearch(e.target.value)} className="pl-9" />
            </div>
          </div>
        )}
        {tab === "tryout" ? (
          held.length === 0 ? (
            <TabEmpty title={t.emptyTryoutTabTitle} description={t.emptyTabDescription} cta={t.emptyCta} />
          ) : shownTryouts.length === 0 ? (
            <EmptyState icon={Search} title={t.noMatchTitle} description={t.noMatchDescription} />
          ) : (
            <div className={gridClass}>
              {shownTryouts.map((e) => {
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
                  <ProductCard
                    key={e.id}
                    icon={BookOpen}
                    tone="peach"
                    chip={
                      <StatusChip tone={state === "completed" ? "success" : state === "in_progress" ? "accent" : "neutral"}>
                        {state === "completed" ? t.stateCompleted : state === "in_progress" ? t.stateInProgress : t.stateNotStarted}
                      </StatusChip>
                    }
                    program={<ProgramBadge code={e.program_code} />}
                    title={e.tryout_title}
                    meta={
                      <>
                        <span className="inline-flex items-center gap-1.5">
                          <Icon icon={ListChecks} size="xs" /> {t.subtestCount(e.subtest_count)}
                        </span>
                        <span className="inline-flex items-center gap-1.5">
                          <Icon icon={Clock} size="xs" /> {t.activeUntil(new Date(e.active_until).toLocaleDateString())}
                        </span>
                      </>
                    }
                    status={latest ? `${t.attemptsDone(done.length)}${latest.score !== null ? ` · ${t.latestScore(latest.score)}` : ""}` : undefined}
                    actions={
                      state === "completed" && latest ? (
                        <>
                          <TextLink href={`/student/tryout/${e.tryout_id}/result?attempt=${latest.session_id}`}>{t.ctaViewResult}</TextLink>
                          <TextLink href={`/student/tryout/${e.tryout_id}`}>{t.ctaHistory}</TextLink>
                        </>
                      ) : (
                        <TextLink href={`/student/tryout/${e.tryout_id}`}>{state === "in_progress" ? t.ctaContinue : t.ctaStart}</TextLink>
                      )
                    }
                  />
                );
              })}
            </div>
          )
        ) : kecTests.length === 0 ? (
          <TabEmpty title={t.emptyKecermatanTabTitle} description={t.emptyTabDescription} cta={t.emptyCta} />
        ) : shownTests.length === 0 ? (
          <EmptyState icon={Search} title={t.noMatchTitle} description={t.noMatchDescription} />
        ) : (
          <div className={gridClass}>
            {shownTests.map((test) => {
              const left = test.attempts_remaining;

              return (
                <ProductCard
                  key={test.test_id}
                  icon={Sparkles}
                  tone="green"
                  chip={<StatusChip tone={left > 0 ? "accent" : "danger"}>{left > 0 ? t.attemptsRemaining(left) : t.quotaExhausted}</StatusChip>}
                  program={<ProgramBadge code={test.program_code} />}
                  title={test.title}
                  meta={
                    <span className="inline-flex items-center gap-1.5">
                      <Icon icon={Sparkles} size="xs" /> {t.kolomSoalStat(test.kolom_count, test.soal_per_kolom)}
                    </span>
                  }
                  actions={<TextLink href={`/student/kecermatan/${test.test_id}`}>{left > 0 ? t.ctaWork : t.ctaViewHistory}</TextLink>}
                />
              );
            })}
          </div>
        )}
      </div>
    </AppShell>
  );
}

function ProductCard({
  icon,
  tone,
  chip,
  program,
  title,
  meta,
  status,
  actions,
}: {
  icon: typeof BookOpen;
  tone: "peach" | "green";
  chip: React.ReactNode;
  program: React.ReactNode;
  title: string;
  meta: React.ReactNode;
  status?: string;
  actions: React.ReactNode;
}) {
  return (
    <article className="flex min-h-64 min-w-0 flex-col rounded-card border border-border bg-surface p-6">
      <div className="mb-4.5 flex items-start justify-between gap-3">
        <span className={`grid size-13 shrink-0 place-items-center rounded-control ${tone === "peach" ? "bg-peach text-accent" : "bg-success-soft text-success-soft-foreground"}`}>
          <Icon icon={icon} size="md" />
        </span>
        {chip}
      </div>
      <div className="mb-3">{program}</div>
      <h3 className="mb-2.5 font-display text-title">{title}</h3>
      <div className="flex flex-wrap gap-x-3 gap-y-1 text-label text-muted">{meta}</div>
      {status && (
        <p className="mt-3 flex items-center gap-1.5 text-micro text-success-soft-foreground">
          <Icon icon={Check} size="xs" />
          {status}
        </p>
      )}
      <div className="mt-auto flex flex-wrap items-center justify-end gap-x-4 pt-5">{actions}</div>
    </article>
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
