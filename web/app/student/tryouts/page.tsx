"use client";

import { Card, Chip } from "@heroui/react";
import { Clock, ListChecks, PackageOpen, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { useUser } from "@/lib/session-context";
import { loadSession } from "@/lib/tryout-session";
import { remainingAttempts } from "@/lib/kecermatan-session";
import { useTryoutsListT } from "@/lib/i18n/dict/tryouts-list";
import {
  formatDate,
  getEnrollmentsForUser,
  getKecermatanAttemptsPurchased,
  getKecermatanEnrollmentsForUser,
  getKecermatanTest,
  getProgram,
  getSubtestsForTryout,
  getTryout,
} from "@/lib/mock-data";
import { useHydrated } from "@/lib/use-hydrated";

type TryoutState = "not_started" | "in_progress" | "completed";

export default function StudentTryoutsPage() {
  const t = useTryoutsListT();
  const user = useUser();
  const hydrated = useHydrated();

  const enrollments = getEnrollmentsForUser(user.id);
  const kecTestIds = [...new Set(getKecermatanEnrollmentsForUser(user.id).map((e) => e.kecermatanTestId))];

  const statuses: Record<string, TryoutState> = {};
  const kecRemaining: Record<string, number> = {};
  if (hydrated) {
    for (const e of enrollments) {
      const session = loadSession(e.tryoutId, user.id);
      statuses[e.tryoutId] = !session ? "not_started" : session.status === "completed" ? "completed" : "in_progress";
    }
    for (const testId of kecTestIds) {
      const purchased = getKecermatanAttemptsPurchased(user.id, testId);
      kecRemaining[testId] = remainingAttempts(testId, user.id, purchased);
    }
  }

  if (enrollments.length === 0 && kecTestIds.length === 0) {
    return (
      <AppShell title={t.pageTitle} description={t.pageDescription} maxWidth="max-w-6xl">
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

  return (
    <AppShell title={t.pageTitle} description={t.pageDescription} maxWidth="max-w-6xl">
      {enrollments.length > 0 && (
        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.tryoutSectionTitle}</p>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {enrollments.map((e) => {
              const tryout = getTryout(e.tryoutId);
              if (!tryout) return null;
              const program = getProgram(tryout.programId);
              const subtests = getSubtestsForTryout(tryout.id);
              const state = statuses[tryout.id] ?? "not_started";
              const cta =
                state === "completed"
                  ? { label: t.ctaViewResult, href: `/student/tryout/${tryout.id}/result` }
                  : state === "in_progress"
                    ? { label: t.ctaContinue, href: `/student/tryout/${tryout.id}` }
                    : { label: t.ctaStart, href: `/student/tryout/${tryout.id}` };

              return (
                <Card key={e.id} className="gap-0 overflow-hidden p-0">
                  <div
                    className="h-20 w-full"
                    style={{ background: `linear-gradient(135deg, ${tryout.coverFrom}, ${tryout.coverTo})` }}
                  />
                  <div className="space-y-3 p-4">
                    <div className="flex items-center justify-between gap-2">
                      {program && <ProgramBadge code={program.code} />}
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
                    <p className="line-clamp-1 text-sm font-semibold text-foreground">{tryout.title}</p>
                    <div className="flex gap-3 text-xs text-muted">
                      <span className="inline-flex items-center gap-1">
                        <ListChecks className="size-3.5" /> {t.subtestCount(subtests.length)}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <Clock className="size-3.5" /> {t.activeUntil(formatDate(e.activeUntil))}
                      </span>
                    </div>
                    <LinkButton href={cta.href} variant={state === "completed" ? "secondary" : "primary"} size="sm" className="w-full">
                      {cta.label}
                    </LinkButton>
                  </div>
                </Card>
              );
            })}
          </div>
        </div>
      )}

      {kecTestIds.length > 0 && (
        <div className={enrollments.length > 0 ? "mt-8" : undefined}>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.kecermatanSectionTitle}</p>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {kecTestIds.map((testId) => {
              const test = getKecermatanTest(testId);
              if (!test) return null;
              const program = getProgram(test.programId);
              const left = kecRemaining[testId] ?? 0;

              return (
                <Card key={testId} className="gap-0 overflow-hidden p-0">
                  <div className="h-20 w-full" style={{ background: `linear-gradient(135deg, ${test.coverFrom}, ${test.coverTo})` }} />
                  <div className="space-y-3 p-4">
                    <div className="flex items-center justify-between gap-2">
                      {program && <ProgramBadge code={program.code} />}
                      <Chip size="sm" className={`font-medium ${left > 0 ? "bg-accent-soft text-accent-soft-foreground" : "bg-danger-soft text-danger-soft-foreground"}`}>
                        {left > 0 ? t.attemptsRemaining(left) : t.quotaExhausted}
                      </Chip>
                    </div>
                    <p className="line-clamp-1 text-sm font-semibold text-foreground">{test.title}</p>
                    <p className="flex items-center gap-1.5 text-xs text-muted">
                      <Sparkles className="size-3.5" /> {t.kolomSoalStat(test.kolomCount, test.soalPerKolom)}
                    </p>
                    <LinkButton href={`/student/kecermatan/${testId}`} variant={left > 0 ? "primary" : "secondary"} size="sm" className="w-full">
                      {left > 0 ? t.ctaWork : t.ctaViewHistory}
                    </LinkButton>
                  </div>
                </Card>
              );
            })}
          </div>
        </div>
      )}
    </AppShell>
  );
}
