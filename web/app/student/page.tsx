"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Skeleton } from "@heroui/react";
import { ArrowRight, Award, BookOpen, ClipboardList, CloudOff, PackageOpen, Sparkles, Trophy, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { EmptyState } from "@/components/empty-state";
import { StatCard } from "@/components/stat-card";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { api, errorText, type AttemptSummary, type Enrollment, type KecermatanStudentTest } from "@/lib/api/client";
import { useUser } from "@/lib/session-context";
import { formatIDR } from "@/lib/format-money";
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

  return (
    <AppShell title={t.pageTitle}>
      <div className="space-y-6">
        <div className="space-y-2">
          <h2 className="font-display text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">{t.greeting(user.name.split(" ")[0])}</h2>
          <p className="text-sm text-muted">{t.subtitle}</p>
        </div>

        {error ? (
          <EmptyState icon={CloudOff} title={t.loadErrorTitle} description={error} action={<Button variant="primary" size="sm" onPress={() => { setError("");setLoaded(null);setReloadKey((key) => key + 1); }}>{t.retry}</Button>} />
        ) : !data ? (
          <div role="status" aria-label={t.loading} className="space-y-6">
            <p className="text-sm text-muted">{t.loading}</p>
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">{[0, 1, 2, 3].map((key) => <Skeleton key={key} className="h-28 rounded-2xl" />)}</div>
            <Skeleton className="h-48 rounded-3xl" />
            <div className="grid gap-5 md:grid-cols-2"><Skeleton className="h-64 rounded-2xl" /><Skeleton className="h-64 rounded-2xl" /></div>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
              <StatCard compact label={t.ownedTryouts} value={String(data.summary.owned_tryouts)} icon={ClipboardList} />
              <StatCard compact label={t.completedTryouts} value={String(data.summary.completed_tryouts)} icon={Trophy} />
              <StatCard compact label={t.averageScore} value={data.summary.average_score === null ? "—" : formatScore(data.summary.average_score)} icon={Award} />
              <StatCard compact label={t.balance} value={formatIDR(data.summary.balance)} icon={Wallet} />
            </div>

            <Card className="flex-col gap-6 rounded-3xl border border-accent/15 bg-accent/5 p-6 shadow-none sm:flex-row sm:items-center sm:justify-between sm:p-8">
              <div className="max-w-lg space-y-3">
                <h3 className="font-display text-xl font-semibold tracking-tight text-foreground sm:text-2xl">{hasContent ? t.continueTitle : t.welcomeTitle}</h3>
                <p className="text-sm leading-relaxed text-muted">{t.welcomeDescription}</p>
                <LinkButton href={hasContent ? "/student/tryouts" : "/catalog"} variant="primary" className="mt-2 min-h-10 w-fit">{hasContent ? t.myTryouts : t.viewCatalog}<ArrowRight className="size-4" /></LinkButton>
              </div>
              <div className="hidden shrink-0 sm:block" aria-hidden="true"><CountdownRing value={76} size={108} strokeWidth={7} animate={false} trackClassName="stroke-accent/10"><Sparkles className="size-8 text-accent" /></CountdownRing></div>
            </Card>

            <div className="grid gap-5 md:grid-cols-2">
              <Card className="gap-5 rounded-2xl border border-separator p-5 shadow-none sm:p-6">
                <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="text-sm font-semibold">{t.myTryouts}</h3>{hasContent && <Link href="/student/tryouts" className="text-xs font-medium text-accent hover:underline">{t.viewAll}</Link>}</div>
                {!hasContent ? <EmptyState icon={PackageOpen} title={t.emptyTitle} description={t.emptyTryoutsDescription} className="border-0 px-2 py-10" action={<LinkButton href="/catalog" variant="ghost" size="sm">{t.viewCatalog}</LinkButton>} /> : (
                  <ul className="space-y-3">
                    {data.enrollments.slice(0, 3).map((enrollment) => (
                      <li key={enrollment.id}><Link href={`/student/tryout/${enrollment.tryout_id}`} className="flex min-h-16 items-center gap-3 rounded-xl border border-border p-3 transition-colors hover:bg-surface-secondary"><BookOpen className="size-4 shrink-0 text-accent" /><div className="min-w-0 flex-1"><p className="break-words text-sm font-medium">{enrollment.tryout_title}</p><p className="mt-1 text-xs text-muted">{enrollment.program_name}</p></div><ArrowRight className="size-4 shrink-0 text-muted" /></Link></li>
                    ))}
                    {data.kecermatan.slice(0, Math.max(0, 3 - data.enrollments.length)).map((test) => (
                      <li key={test.test_id}><Link href={`/student/kecermatan/${test.test_id}`} className="flex min-h-16 items-center gap-3 rounded-xl border border-border p-3 transition-colors hover:bg-surface-secondary"><Sparkles className="size-4 shrink-0 text-accent" /><div className="min-w-0 flex-1"><p className="break-words text-sm font-medium">{test.title}</p><p className="mt-1 text-xs text-muted">{t.kecermatan}</p></div><ArrowRight className="size-4 shrink-0 text-muted" /></Link></li>
                    ))}
                  </ul>
                )}
              </Card>
              <Card className="gap-5 rounded-2xl border border-separator p-5 shadow-none sm:p-6">
                <h3 className="text-sm font-semibold">{t.latestResults}</h3>
                {recent.length === 0 ? <EmptyState icon={Award} title={t.noResults} description={t.noResultsDescription} className="border-0 px-2 py-10" /> : (
                  <ul className="space-y-3">
                    {recent.map((attempt) => {
                      const enrollment = enrollmentByTryout.get(attempt.tryout_id);
                      return <li key={attempt.session_id}><Link href={`/student/tryout/${attempt.tryout_id}/result`} className="flex items-center gap-3 rounded-xl border border-border p-3 transition-colors hover:bg-surface-secondary"><div className="min-w-0 flex-1">{enrollment && <ProgramBadge code={enrollment.program_code} />}<p className="mt-1 break-words text-sm font-medium">{enrollment?.tryout_title ?? t.genericTryout}</p><p className="mt-1 text-xs text-muted">{t.attempt(attempt.attempt_number)}{attempt.completed_at && <> · {new Date(attempt.completed_at).toLocaleDateString(localeName)}</>}</p></div><span className="font-display text-xl font-semibold text-foreground tabular-nums">{formatScore(attempt.score!)}</span></Link></li>;
                    })}
                  </ul>
                )}
              </Card>
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
