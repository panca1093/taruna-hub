"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Skeleton } from "@heroui/react";
import type { LucideIcon } from "lucide-react";
import { ArrowRight, Award, BookOpen, Clock, CloudOff, PackageOpen, Sparkles, Trophy, TrendingUp, BarChart3 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
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

  return (
    <AppShell title={t.pageTitle}>
      <div className="space-y-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="space-y-2">
            <h2 className="font-display text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">{t.greeting(user.name.split(" ")[0])}</h2>
            <p className="text-sm text-muted">{t.subtitle}</p>
          </div>
          <LinkButton href="/catalog" variant="outline" className="min-h-10">{t.viewCatalog}<ArrowRight className="size-4" /></LinkButton>
        </div>

        {error ? (
          <EmptyState icon={CloudOff} title={t.loadErrorTitle} description={error} action={<Button variant="primary" size="sm" onPress={() => { setError("");setLoaded(null);setReloadKey((key) => key + 1); }}>{t.retry}</Button>} />
        ) : !data ? (
          <div role="status" aria-label={t.loading} className="space-y-6">
            <p className="text-sm text-muted">{t.loading}</p>
            <div className="grid gap-5 md:grid-cols-2"><Skeleton className="h-64 rounded-3xl" /><Skeleton className="h-64 rounded-3xl" /></div>
            <Skeleton className="h-20 rounded-2xl" />
            <div className="grid gap-5 lg:grid-cols-3"><Skeleton className="h-64 rounded-2xl lg:col-span-2" /><Skeleton className="h-64 rounded-2xl" /></div>
          </div>
        ) : (
          <>
            <div className="grid gap-5 md:grid-cols-2">
              <Card className="min-h-64 justify-between gap-8 rounded-3xl border-0 bg-peach p-6 shadow-none sm:p-8">
                <div className="space-y-3">
                  <span className="inline-flex items-center gap-2 text-xs font-medium text-foreground/80"><BookOpen className="size-4" aria-hidden />{t.tryoutKicker}</span>
                  <h3 className="font-display text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">{featuredTryout ? featuredTryout.tryout_title : t.tryoutPromoTitle}</h3>
                  <p className="max-w-md text-sm leading-relaxed text-foreground/75">{featuredTryout ? t.tryoutOwnedDescription : t.tryoutPromoDescription}</p>
                </div>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <span className="inline-flex items-center gap-2 text-xs text-foreground/75">{featuredTryout && <><Clock className="size-4" aria-hidden />{t.subtestCount(featuredTryout.subtest_count)}</>}</span>
                  <LinkButton href={featuredTryout ? `/student/tryout/${featuredTryout.tryout_id}` : "/catalog"} variant="primary" className="min-h-10">{featuredTryout ? t.openTryout : t.viewCatalog}<ArrowRight className="size-4" /></LinkButton>
                </div>
              </Card>
              <Card className="min-h-64 justify-between gap-8 rounded-3xl border-0 bg-accent-soft p-6 shadow-none sm:p-8">
                <div className="space-y-3">
                  <span className="inline-flex items-center gap-2 text-xs font-medium text-foreground/80"><Sparkles className="size-4" aria-hidden />{t.kecermatanKicker}</span>
                  <h3 className="font-display text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">{featuredTest ? featuredTest.title : t.kecermatanPromoTitle}</h3>
                  <p className="max-w-md text-sm leading-relaxed text-foreground/75">{t.kecermatanPromoDescription}</p>
                </div>
                <div className="flex flex-wrap items-end justify-between gap-3">
                  {featuredTest ? (
                    <p className="flex items-baseline gap-2"><span className="font-display text-4xl font-semibold tabular-nums text-accent">{featuredTest.attempts_remaining}</span><span className="text-xs text-foreground/75">{t.attemptsLeft}</span></p>
                  ) : <span />}
                  <LinkButton href={featuredTest ? `/student/kecermatan/${featuredTest.test_id}` : "/catalog"} variant="primary" className="min-h-10">{featuredTest ? t.openTest : t.viewCatalog}<ArrowRight className="size-4" /></LinkButton>
                </div>
              </Card>
            </div>

            <Card className="rounded-2xl border border-separator p-0 shadow-none">
              <dl className="grid grid-cols-2 divide-separator lg:grid-cols-4 lg:divide-x">
                {([
                  [t.ownedTryouts, String(data.summary.owned_tryouts), BookOpen],
                  [t.attemptsQuota, String(quota), Sparkles],
                  [t.completedTryouts, String(data.summary.completed_tryouts), Trophy],
                  [t.averageScore, data.summary.average_score === null ? "—" : formatScore(data.summary.average_score), TrendingUp],
                ] as [string, string, LucideIcon][]).map(([label, value, StatIcon]) => {
                  return (
                    <div key={label} className="flex items-center gap-3 p-4 sm:p-5">
                      <span className="inline-flex size-10 shrink-0 items-center justify-center rounded-xl bg-surface-secondary text-muted"><StatIcon className="size-4.5" aria-hidden /></span>
                      <div className="min-w-0"><dd className="font-display text-xl font-semibold tabular-nums text-foreground">{value}</dd><dt className="text-xs text-muted">{label}</dt></div>
                    </div>
                  );
                })}
              </dl>
            </Card>

            <div className="grid gap-5 lg:grid-cols-3">
              <section className="space-y-4 lg:col-span-2" aria-labelledby="practice-heading">
                <div className="flex flex-wrap items-center justify-between gap-3"><h3 id="practice-heading" className="text-base font-semibold">{t.myPractice}</h3>{hasContent && <Link href="/student/tryouts" className="text-xs font-medium text-accent hover:underline">{t.viewAll}</Link>}</div>
                {!hasContent ? <EmptyState icon={PackageOpen} title={t.emptyTitle} description={t.emptyTryoutsDescription} action={<LinkButton href="/catalog" variant="ghost" size="sm">{t.viewCatalog}</LinkButton>} /> : (
                  <ul className="space-y-3">
                    {data.enrollments.slice(0, 3).map((enrollment) => (
                      <li key={enrollment.id}><Link href={`/student/tryout/${enrollment.tryout_id}`} className="flex min-h-16 items-center gap-3 rounded-2xl border border-separator bg-surface p-3 transition-colors hover:bg-surface-secondary"><span className="inline-flex size-10 shrink-0 items-center justify-center rounded-xl bg-peach text-foreground"><BookOpen className="size-4.5" aria-hidden /></span><div className="min-w-0 flex-1"><p className="break-words text-sm font-medium">{enrollment.tryout_title}</p><p className="mt-1 text-xs text-muted">{t.accessUntil(formatDate(enrollment.active_until))}</p></div><ArrowRight className="size-4 shrink-0 text-muted" aria-hidden /></Link></li>
                    ))}
                    {data.kecermatan.slice(0, Math.max(1, 4 - Math.min(3, data.enrollments.length))).map((test) => (
                      <li key={test.test_id}><Link href={`/student/kecermatan/${test.test_id}`} className="flex min-h-16 items-center gap-3 rounded-2xl border border-separator bg-surface p-3 transition-colors hover:bg-surface-secondary"><span className="inline-flex size-10 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent"><Sparkles className="size-4.5" aria-hidden /></span><div className="min-w-0 flex-1"><p className="break-words text-sm font-medium">{test.title}</p><p className="mt-1 text-xs text-muted">{t.attemptsAvailable(test.attempts_remaining)}</p></div><ArrowRight className="size-4 shrink-0 text-muted" aria-hidden /></Link></li>
                    ))}
                  </ul>
                )}
              </section>
              <section className="space-y-4" aria-labelledby="results-heading">
                <h3 id="results-heading" className="text-base font-semibold">{t.latestResults}</h3>
                {recent.length === 0 ? <EmptyState icon={Award} title={t.noResults} description={t.noResultsDescription} /> : (
                  <ul className="divide-y divide-separator rounded-2xl border border-separator bg-surface">
                    {recent.map((attempt) => {
                      const enrollment = enrollmentByTryout.get(attempt.tryout_id);
                      return <li key={attempt.session_id}><Link href={`/student/tryout/${attempt.tryout_id}/result?attempt=${attempt.session_id}`} className="flex items-center gap-4 p-4 transition-colors hover:bg-surface-secondary"><span className="min-w-12 font-display text-2xl font-semibold text-foreground tabular-nums">{formatScore(attempt.score!)}</span><div className="min-w-0 flex-1">{enrollment && <ProgramBadge code={enrollment.program_code} />}<p className="mt-1 break-words text-sm font-medium">{enrollment?.tryout_title ?? t.genericTryout}</p><p className="mt-1 text-xs text-muted">{t.attempt(attempt.attempt_number)}{attempt.completed_at && <> · {formatDate(attempt.completed_at)}</>}</p></div><ArrowRight className="size-4 shrink-0 text-muted" aria-hidden /></Link></li>;
                    })}
                  </ul>
                )}
                {recent.length > 0 && <p className="flex items-start gap-2 text-xs leading-relaxed text-muted"><BarChart3 className="mt-0.5 size-4 shrink-0" aria-hidden />{t.separateResultsNote}</p>}
              </section>
            </div>

            {!hasContent && <Card className="gap-5 rounded-2xl border border-separator p-5 shadow-none sm:p-6"><h3 className="text-sm font-semibold">{t.nextSteps}</h3><div className="grid gap-6 sm:grid-cols-3">{[
              [t.stepChoose, t.stepChooseDescription], [t.stepPractice, t.stepPracticeDescription], [t.stepReview, t.stepReviewDescription],
            ].map(([title, description], index) => <div key={title}><span className="mb-3 flex size-6 items-center justify-center rounded-full border border-border text-xs text-muted">{index + 1}</span><p className="text-xs font-medium">{title}</p><p className="mt-2 text-xs leading-relaxed text-muted">{description}</p></div>)}</div></Card>}
          </>
        )}
      </div>
    </AppShell>
  );
}
