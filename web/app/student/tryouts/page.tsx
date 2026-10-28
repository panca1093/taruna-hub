"use client";

import { useEffect, useState } from "react";
import { Button, Card, Chip } from "@heroui/react";
import { Clock, CloudOff, ListChecks, PackageOpen, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { useUser } from "@/lib/session-context";
import { remainingAttempts } from "@/lib/kecermatan-session";
import { useTryoutsListT } from "@/lib/i18n/dict/tryouts-list";
import { api, type AttemptSummary, type Enrollment } from "@/lib/api/client";
import {
  getKecermatanAttemptsPurchased,
  getKecermatanEnrollmentsForUser,
  getKecermatanTest,
  getProgram,
} from "@/lib/mock-data";
import { useHydrated } from "@/lib/use-hydrated";

type TryoutState = "not_started" | "in_progress" | "completed";

export default function StudentTryoutsPage() {
  const t = useTryoutsListT();
  const user = useUser();
  const hydrated = useHydrated();

  const [enrollments, setEnrollments] = useState<Enrollment[] | null>(null);
  const [attempts, setAttempts] = useState<AttemptSummary[]>([]);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.orders.enrollments(), api.runtime.attempts()])
      .then(([list, history]) => {
        if (cancelled) return;
        setEnrollments(list.enrollments);
        setAttempts(history.attempts);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const held = enrollments ?? [];
  // Kecermatan purchases arrive with P18; until then this still reads the mock layer.
  const kecTestIds = [...new Set(getKecermatanEnrollmentsForUser(user.id).map((e) => e.kecermatanTestId))];

  const statuses: Record<string, TryoutState> = {};
  const kecRemaining: Record<string, number> = {};
  if (hydrated) {
    for (const e of held) {
      const own = attempts.filter((a) => a.tryout_id === e.tryout_id);
      statuses[e.tryout_id] = own.some((a) => a.status === "in_progress")
        ? "in_progress"
        : own.length > 0
          ? "completed"
          : "not_started";
    }
    for (const testId of kecTestIds) {
      const purchased = getKecermatanAttemptsPurchased(user.id, testId);
      kecRemaining[testId] = remainingAttempts(testId, user.id, purchased);
    }
  }

  if (loadFailed || enrollments === null) {
    return (
      <AppShell title={t.pageTitle} description={t.pageDescription} maxWidth="max-w-6xl">
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

  if (held.length === 0 && kecTestIds.length === 0) {
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
      {held.length > 0 && (
        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.tryoutSectionTitle}</p>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {held.map((e) => {
              const state = statuses[e.tryout_id] ?? "not_started";
              const cta =
                state === "completed"
                  ? { label: t.ctaViewResult, href: `/student/tryout/${e.tryout_id}/result` }
                  : state === "in_progress"
                    ? { label: t.ctaContinue, href: `/student/tryout/${e.tryout_id}` }
                    : { label: t.ctaStart, href: `/student/tryout/${e.tryout_id}` };

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
                    <div className="flex gap-3 text-xs text-muted">
                      <span className="inline-flex items-center gap-1">
                        <ListChecks className="size-3.5" /> {t.subtestCount(e.subtest_count)}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <Clock className="size-3.5" /> {t.activeUntil(new Date(e.active_until).toLocaleDateString())}
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
        <div className={held.length > 0 ? "mt-8" : undefined}>
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
