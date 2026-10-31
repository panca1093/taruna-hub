"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Skeleton } from "@heroui/react";
import { ArrowRight, Award, BookOpen, Clock, CloudOff, PackageOpen, Sparkles, Trophy, TrendingUp, BarChart3 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { Icon } from "@/components/icon";
import { Metric, MetricStrip } from "@/components/metric-strip";
import { CardKicker, Panel, SectionHeader } from "@/components/page-blocks";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { api, errorText, type AttemptSummary, type Enrollment, type KecermatanStudentTest } from "@/lib/api/client";
import { useUser } from "@/lib/session-context";
import { useLocale } from "@/lib/i18n/locale-context";
import { useStudentDashboardT } from "@/lib/i18n/dict/student-dashboard";

type DashboardData = {
  userId: string;
  summary: Awaited<ReturnType<typeof api.studentDashboard>>;
  enrollments: Enrollment[];
  attempts: AttemptSummary[];
  kecermatan: KecermatanStudentTest[];
};

export default function StudentDashboardPage() {
  const user = useUser();
  const t = useStudentDashboardT();
  const { locale } = useLocale();
  const [loaded, setLoaded] = useState<DashboardData | null>(null);
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const data = loaded?.userId === user.id ? loaded : null;

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.studentDashboard(), api.orders.enrollments(), api.runtime.attempts(), api.kecermatanRuntime.tests()])
      .then(([summary, enrollments, history, kecermatan]) => {
        if (!cancelled) setLoaded({ userId: user.id, summary, enrollments: enrollments.enrollments, attempts: history.attempts, kecermatan: kecermatan.tests });
      })
      .catch((err: unknown) => { if (!cancelled) setError(errorText(err, t.loadErrorDescription)); });
    return () => { cancelled = true; };
  }, [reloadKey, user.id, t.loadErrorDescription]);

  const localeName = locale === "id" ? "id-ID" : "en-US";
  const formatScore = (score: number) => score.toLocaleString(localeName, { maximumFractionDigits: 2 });
  const formatDate = (iso: string) => new Date(iso).toLocaleDateString(localeName, { day: "numeric", month: "short", year: "numeric" });
  const completed = [...(data?.attempts ?? [])].filter((attempt) => attempt.status === "completed" && attempt.score !== null)
    .sort((a, b) => (b.completed_at ?? "").localeCompare(a.completed_at ?? ""));
  const seen = new Set<string>();
  const recent = completed.filter((attempt) => {
    if (seen.has(attempt.tryout_id)) return false;
    seen.add(attempt.tryout_id);
    return true;
  }).slice(0, 5);
  const enrollmentByTryout = new Map(data?.enrollments.map((enrollment) => [enrollment.tryout_id, enrollment]));
  const hasContent = !!data && (data.enrollments.length > 0 || data.kecermatan.length > 0);
  const featuredTryout = data?.enrollments[0];
  const featuredTest = data?.kecermatan.find((test) => test.attempts_remaining > 0) ?? data?.kecermatan[0];
  const quota = data?.kecermatan.reduce((sum, test) => sum + test.attempts_remaining, 0) ?? 0;

  const tileClass = "grid size-11 shrink-0 place-items-center rounded-control";
  const rowClass = "flex min-h-21 items-center gap-3.5 rounded-tile border border-border bg-surface px-4.5 py-4 transition-colors hover:border-rose";

  return (
    <AppShell
      title={t.greeting(user.name.split(" ")[0])}
      description={t.subtitle}
      actions={<LinkButton href="/catalog" variant="outline" className="min-h-10">{t.viewCatalog}<Icon icon={ArrowRight} size="xs" /></LinkButton>}
    >
      <div className="space-y-6">
        {error ? (
          <EmptyState icon={CloudOff} title={t.loadErrorTitle} description={error} action={<Button variant="primary" size="sm" onPress={() => { setError("");setLoaded(null);setReloadKey((key) => key + 1); }}>{t.retry}</Button>} />
        ) : !data ? (
          <div role="status" aria-label={t.loading} className="space-y-6">
            <p className="text-small text-muted">{t.loading}</p>
            <div className="grid gap-5 md:grid-cols-2"><Skeleton className="h-66 rounded-hero" /><Skeleton className="h-66 rounded-hero" /></div>
            <Skeleton className="h-20 rounded-tile" />
            <div className="grid gap-7 lg:grid-cols-[minmax(0,1.28fr)_minmax(0,0.9fr)]"><Skeleton className="h-64 rounded-tile" /><Skeleton className="h-64 rounded-tile" /></div>
          </div>
        ) : (
          <>
            <div className="grid gap-5 md:grid-cols-2">
              <Panel tone="peach" radius="hero" as="section" className="flex min-h-66 flex-col sm:px-7.5">
                <CardKicker icon={BookOpen}>{t.tryoutKicker}</CardKicker>
                <h2 className="mb-2.5 max-w-md font-display text-hero-lg">{featuredTryout ? featuredTryout.tryout_title : t.tryoutPromoTitle}</h2>
                <p className="max-w-md text-small text-muted">{featuredTryout ? t.tryoutOwnedDescription : t.tryoutPromoDescription}</p>
                <div className="mt-auto flex flex-wrap items-center justify-between gap-3 pt-6">
                  <span className="flex items-center gap-2 text-label text-muted">{featuredTryout && <><Icon icon={Clock} size="sm" />{t.subtestCount(featuredTryout.subtest_count)}</>}</span>
                  <LinkButton href={featuredTryout ? `/student/tryout/${featuredTryout.tryout_id}` : "/catalog"} variant="primary" className="min-h-10">{featuredTryout ? t.openTryout : t.viewCatalog}<Icon icon={ArrowRight} size="xs" /></LinkButton>
                </div>
              </Panel>
              <Panel tone="soft" radius="hero" as="section" className="flex min-h-66 flex-col sm:px-7.5">
                <CardKicker icon={Sparkles}>{t.kecermatanKicker}</CardKicker>
                <h2 className="mb-2.5 max-w-md font-display text-hero-lg">{featuredTest ? featuredTest.title : t.kecermatanPromoTitle}</h2>
                <p className="max-w-md text-small text-muted">{t.kecermatanPromoDescription}</p>
                <div className="mt-auto flex flex-wrap items-end justify-between gap-3 pt-6">
                  {featuredTest ? (
                    <p className="flex items-baseline gap-1.5"><span className="font-display text-metric-lg tabular-nums text-accent">{featuredTest.attempts_remaining}</span><span className="text-label text-muted">{t.attemptsLeft}</span></p>
                  ) : <span />}
                  <LinkButton href={featuredTest ? `/student/kecermatan/${featuredTest.test_id}` : "/catalog"} variant="primary" className="min-h-10">{featuredTest ? t.openTest : t.viewCatalog}<Icon icon={ArrowRight} size="xs" /></LinkButton>
                </div>
              </Panel>
            </div>

            <MetricStrip>
              <Metric icon={BookOpen} value={String(data.summary.owned_tryouts)} label={t.ownedTryouts} />
              <Metric icon={Sparkles} value={String(quota)} label={t.attemptsQuota} />
              <Metric icon={Trophy} value={String(data.summary.completed_tryouts)} label={t.completedTryouts} />
              <Metric icon={TrendingUp} value={data.summary.average_score === null ? "—" : formatScore(data.summary.average_score)} label={t.averageScore} />
            </MetricStrip>

            <div className="grid gap-7 lg:grid-cols-[minmax(0,1.28fr)_minmax(0,0.9fr)] lg:gap-9">
              <section>
                <SectionHeader title={t.myPractice} href={hasContent ? "/student/tryouts" : undefined} linkLabel={t.viewAll} />
                {!hasContent ? <EmptyState icon={PackageOpen} title={t.emptyTitle} description={t.emptyTryoutsDescription} action={<LinkButton href="/catalog" variant="ghost" size="sm">{t.viewCatalog}</LinkButton>} /> : (
                  <ul className="grid gap-2.5">
                    {data.enrollments.slice(0, 3).map((enrollment) => (
                      <li key={enrollment.id}><Link href={`/student/tryout/${enrollment.tryout_id}`} className={rowClass}><span className={`${tileClass} bg-peach text-accent`}><Icon icon={BookOpen} size="md" /></span><div className="min-w-0 flex-1"><p className="break-words text-caption font-medium">{enrollment.tryout_title}</p><p className="mt-1 text-label text-muted">{t.accessUntil(formatDate(enrollment.active_until))}</p></div><Icon icon={ArrowRight} size="sm" className="text-muted" /></Link></li>
                    ))}
                    {data.kecermatan.slice(0, Math.max(1, 4 - Math.min(3, data.enrollments.length))).map((test) => (
                      <li key={test.test_id}><Link href={`/student/kecermatan/${test.test_id}`} className={rowClass}><span className={`${tileClass} bg-success-soft text-success-soft-foreground`}><Icon icon={Sparkles} size="md" /></span><div className="min-w-0 flex-1"><p className="break-words text-caption font-medium">{test.title}</p><p className="mt-1 text-label text-muted">{t.attemptsAvailable(test.attempts_remaining)}</p></div><Icon icon={ArrowRight} size="sm" className="text-muted" /></Link></li>
                    ))}
                  </ul>
                )}
              </section>
              <section>
                <SectionHeader title={t.latestResults} />
                {recent.length === 0 ? <EmptyState icon={Award} title={t.noResults} description={t.noResultsDescription} /> : (
                  <ul>
                    {recent.map((attempt) => {
                      const enrollment = enrollmentByTryout.get(attempt.tryout_id);
                      return <li key={attempt.session_id}><Link href={`/student/tryout/${attempt.tryout_id}/result?attempt=${attempt.session_id}`} className="flex items-center gap-3 border-b border-border py-4 transition-colors hover:text-accent"><span className="min-w-12 font-display text-heading tabular-nums">{formatScore(attempt.score!)}</span><div className="min-w-0 flex-1">{enrollment && <ProgramBadge code={enrollment.program_code} />}<p className="mt-1 break-words text-label font-medium">{enrollment?.tryout_title ?? t.genericTryout}</p><p className="mt-1 text-micro text-muted">{t.attempt(attempt.attempt_number)}{attempt.completed_at && <> · {formatDate(attempt.completed_at)}</>}</p></div><Icon icon={ArrowRight} size="xs" className="text-muted" /></Link></li>;
                    })}
                  </ul>
                )}
                {recent.length > 0 && <p className="mt-5 flex items-start gap-2.5 text-label leading-relaxed text-muted"><Icon icon={BarChart3} size="sm" className="mt-0.5 text-rose" />{t.separateResultsNote}</p>}
              </section>
            </div>

            {!hasContent && <Panel tone="plain" radius="tile"><h2 className="mb-5 font-display text-heading-sm">{t.nextSteps}</h2><div className="grid gap-6 sm:grid-cols-3">{[
              [t.stepChoose, t.stepChooseDescription], [t.stepPractice, t.stepPracticeDescription], [t.stepReview, t.stepReviewDescription],
            ].map(([title, description], index) => <div key={title}><span className="mb-3 grid size-6 place-items-center rounded-full border border-border text-label text-muted">{index + 1}</span><p className="text-caption font-medium">{title}</p><p className="mt-2 text-label leading-relaxed text-muted">{description}</p></div>)}</div></Panel>}
          </>
        )}
      </div>
    </AppShell>
  );
}
