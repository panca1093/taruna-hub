"use client";

import Link from "next/link";
import { Card } from "@heroui/react";
import { Award, ClipboardList, PackageOpen, Trophy, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { StatCard } from "@/components/stat-card";
import { DonutStat } from "@/components/charts";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { useUser } from "@/lib/session-context";
import { loadSession } from "@/lib/tryout-session";
import { formatDateTime, formatIDR, getBalance, getEnrollmentsForUser, getProgram, getResultsForUser, getTryout } from "@/lib/mock-data";
import { useStudentDashboardT } from "@/lib/i18n/dict/student-dashboard";
import { useHydrated } from "@/lib/use-hydrated";

type TryoutState = "not_started" | "in_progress" | "completed";

export default function StudentDashboardPage() {
  const user = useUser();
  const t = useStudentDashboardT();
  const hydrated = useHydrated();

  const enrollments = getEnrollmentsForUser(user.id);
  const results = [...getResultsForUser(user.id)].sort((a, b) => (a.completedAt < b.completedAt ? 1 : -1));

  const statuses: Record<string, TryoutState> = {};
  if (hydrated) {
    for (const e of enrollments) {
      const session = loadSession(e.tryoutId, user.id);
      statuses[e.tryoutId] = !session ? "not_started" : session.status === "completed" ? "completed" : "in_progress";
    }
  }

  const notStarted = enrollments.filter((e) => (statuses[e.tryoutId] ?? "not_started") === "not_started").length;
  const inProgress = enrollments.filter((e) => statuses[e.tryoutId] === "in_progress").length;
  const completed = enrollments.filter((e) => statuses[e.tryoutId] === "completed").length;
  const avgScore = results.length === 0 ? null : Math.round(results.reduce((s, r) => s + r.totalScore, 0) / results.length);

  return (
    <AppShell title={t.greeting(user.name.split(" ")[0])} description={t.subtitle} maxWidth="max-w-6xl">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label={t.ownedTryouts} value={String(enrollments.length)} icon={ClipboardList} />
        <StatCard label={t.completedTryouts} value={String(completed)} icon={Trophy} />
        <StatCard label={t.averageScore} value={avgScore === null ? "—" : String(avgScore)} icon={Award} />
        <StatCard label={t.balance} value={formatIDR(getBalance(user))} icon={Wallet} />
      </div>

      {enrollments.length === 0 ? (
        <div className="mt-6">
          <EmptyState
            icon={PackageOpen}
            title={t.emptyTitle}
            description={t.emptyDescription}
            action={
              <LinkButton href="/catalog" variant="primary" size="sm">
                {t.viewCatalog}
              </LinkButton>
            }
          />
        </div>
      ) : (
        <>
          <div className="mt-6 grid gap-4 lg:grid-cols-3">
            <Card className="gap-4 p-5">
              <div>
                <p className="text-sm font-semibold text-foreground">{t.tryoutStatus}</p>
                <p className="text-xs text-muted">{t.ownedCountDescription(enrollments.length)}</p>
              </div>
              <DonutStat
                segments={[
                  { label: t.segmentCompleted, value: completed, className: "stroke-success" },
                  { label: t.segmentInProgress, value: inProgress, className: "stroke-accent" },
                  { label: t.segmentNotStarted, value: notStarted, className: "stroke-default" },
                ]}
              />
            </Card>

            <Card className="gap-3 p-5 lg:col-span-2">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-semibold text-foreground">{t.latestResults}</p>
                  <p className="text-xs text-muted">{t.latestResultsDescription}</p>
                </div>
                <Link href="/student/tryouts" className="text-sm font-medium text-accent hover:underline">
                  {t.viewAll}
                </Link>
              </div>
              {results.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted">{t.noResults}</p>
              ) : (
                <ul className="space-y-2">
                  {results.slice(0, 5).map((r) => {
                    const tryout = getTryout(r.tryoutId);
                    const program = tryout ? getProgram(tryout.programId) : undefined;
                    if (!tryout) return null;
                    return (
                      <li key={r.id}>
                        <Link
                          href={`/student/tryout/${tryout.id}/result`}
                          className="flex items-center justify-between gap-3 rounded-lg border border-border px-3.5 py-2.5 text-sm transition-colors hover:border-accent/40 hover:bg-surface-secondary"
                        >
                          <div className="flex min-w-0 items-center gap-2.5">
                            {program && <ProgramBadge code={program.code} />}
                            <span className="truncate font-medium text-foreground">{tryout.title}</span>
                          </div>
                          <div className="flex shrink-0 items-center gap-3 text-xs text-muted">
                            <span>{t.attempt(r.attemptNumber)}</span>
                            <span className="font-mono font-semibold text-foreground">{r.totalScore}</span>
                            <span className="hidden sm:inline">{formatDateTime(r.completedAt)}</span>
                          </div>
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              )}
            </Card>
          </div>

          <div className="mt-4 flex flex-wrap gap-2">
            <LinkButton href="/student/tryouts" variant="secondary" size="sm">
              {t.myTryouts}
            </LinkButton>
            <LinkButton href="/catalog" variant="secondary" size="sm">
              {t.viewCatalog}
            </LinkButton>
            <LinkButton href="/student/orders" variant="secondary" size="sm">
              {t.orderHistory}
            </LinkButton>
          </div>
        </>
      )}
    </AppShell>
  );
}
