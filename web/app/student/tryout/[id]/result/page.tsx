"use client";

import { Suspense, use, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { ArrowRight, Clock, CloudOff, Lock } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { api, ApiError, type StudentTryout, type TryoutResultDetail } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useLocale } from "@/lib/i18n/locale-context";
import { useTryoutResultT } from "@/lib/i18n/dict/tryout-result";

type LoadState = "loading" | "ready" | "failed" | "unavailable" | "notReady";

function ResultContent({ tryoutId, attemptParam }: { tryoutId: string; attemptParam: string | null }) {
  const router = useRouter();
  const t = useTryoutResultT();
  const { locale } = useLocale();
  const formatDate = (iso: string) =>
    new Intl.DateTimeFormat(locale === "id" ? "id-ID" : "en-US", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
      new Date(iso),
    );
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [result, setResult] = useState<TryoutResultDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const loaded = await api.runtime.overview(tryoutId);
      const sessionId = attemptParam ?? loaded.attempts.find((a) => a.status === "completed")?.session_id ?? null;
      if (!sessionId) return { loaded, res: null };
      const res = await api.runtime.result(sessionId);
      return { loaded, res };
    };
    load()
      .then(({ loaded, res }) => {
        if (cancelled) return;
        if (!res) {
          router.replace(`/student/tryout/${tryoutId}`);
          return;
        }
        // A session of another tryout is never shown under this tryout's URL.
        if (res.tryout_id !== tryoutId) {
          setState("unavailable");
          return;
        }
        setOverview(loaded);
        setResult(res);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 409 && err.code === "result_not_ready") setState("notReady");
        else if (err instanceof ApiError && (err.status === 404 || err.status === 403)) setState("unavailable");
        else setState("failed");
      });
    return () => {
      cancelled = true;
    };
  }, [tryoutId, attemptParam, router, reloadKey]);

  if (state === "failed") {
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
  if (state === "unavailable" || state === "notReady") {
    return (
      <AppShell title={t.pageTitle}>
        <EmptyState
          icon={Lock}
          title={state === "notReady" ? t.notReadyTitle : t.unavailableTitle}
          description={state === "notReady" ? t.notReadyDescription : t.unavailableDescription}
          action={
            <LinkButton href={`/student/tryout/${tryoutId}`} variant="secondary" size="sm">
              {t.toDetail}
            </LinkButton>
          }
        />
      </AppShell>
    );
  }
  if (state === "loading" || !result || !overview) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const correct = result.subtests.reduce((sum, s) => sum + s.correct_count, 0);
  const total = result.subtests.reduce((sum, s) => sum + s.question_count, 0);
  const completed = overview.attempts.filter((a) => a.status === "completed");
  const reviewHref = `/student/tryout/${tryoutId}/review?attempt=${result.session_id}`;

  return (
    <AppShell title={t.pageTitle} description={overview.title}>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">{t.heading(result.attempt_number)}</h1>
            <p className="mt-1 text-sm text-muted">{t.headingMeta(overview.title, formatDate(result.completed_at))}</p>
          </div>
          {result.explanations_released ? (
            <LinkButton href={reviewHref} variant="primary" size="md">
              {t.viewReview} <ArrowRight className="size-4" />
            </LinkButton>
          ) : (
            <Button variant="secondary" isDisabled>
              <Clock className="size-4" /> {t.reviewHeld}
            </Button>
          )}
        </div>

        <div className="grid gap-4 lg:grid-cols-4">
          <section className="flex flex-col items-center justify-center rounded-2xl bg-accent-soft p-6 text-center text-accent-soft-foreground lg:row-span-2">
            <p className="text-sm">{t.scoreLabel}</p>
            <p className="font-display mt-2 text-6xl font-semibold text-foreground">{result.score}</p>
            <p className="mt-1 text-xs">{t.scoreOutOf}</p>
          </section>
          <Card className="gap-0 p-5">
            <p className="font-display text-3xl font-semibold text-foreground">{correct}</p>
            <p className="text-xs text-muted">{t.statCorrect}</p>
          </Card>
          <Card className="gap-0 p-5">
            <p className="font-display text-3xl font-semibold text-foreground">{total}</p>
            <p className="text-xs text-muted">{t.statQuestions}</p>
          </Card>
          <Card className="gap-0 p-5">
            <p className="font-display text-3xl font-semibold text-foreground">
              {t.rankValue(result.ranking.rank, result.ranking.participant_count)}
            </p>
            <p className="text-xs text-muted">{t.statRank}</p>
            <p className="mt-1 text-xs text-muted">{t.rankNote(result.ranking.average_score, result.ranking.attempts_counted)}</p>
          </Card>
          <Card className="gap-0 p-5 lg:col-span-3">
            {result.explanations_released ? (
              <>
                <h2 className="font-display text-lg font-semibold text-foreground">{t.learnTitle}</h2>
                <p className="mt-1 text-sm text-muted">{t.learnBody}</p>
              </>
            ) : (
              <div className="flex items-start gap-3">
                <Clock className="mt-0.5 size-4 shrink-0 text-muted" />
                <div>
                  <h2 className="font-display text-lg font-semibold text-foreground">{t.heldTitle}</h2>
                  <p className="mt-1 text-sm text-muted">{t.heldBody}</p>
                  {result.explanation_release_at && (
                    <p className="mt-1 text-sm font-medium text-foreground">{t.heldUntil(formatDate(result.explanation_release_at))}</p>
                  )}
                </div>
              </div>
            )}
          </Card>
        </div>

        <Card className="gap-0 p-6">
          <h2 className="font-display text-lg font-semibold text-foreground">{t.subtestScoresTitle}</h2>
          <ul className="mt-4 space-y-4">
            {result.subtests.map((s) => (
              <li key={s.subtest_id}>
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="text-foreground">{s.name}</span>
                  <span className="shrink-0 font-semibold text-foreground">
                    {s.score} / 100 <span className="ml-2 text-xs font-normal text-muted">{t.correctCount(s.correct_count, s.question_count)}</span>
                  </span>
                </div>
                <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-surface-secondary" role="presentation">
                  <div className="h-full rounded-full bg-accent" style={{ width: `${Math.min(100, Math.max(0, s.score))}%` }} />
                </div>
              </li>
            ))}
          </ul>
        </Card>

        {completed.length > 0 && (
          <Card className="gap-0 p-6">
            <h2 className="font-display text-lg font-semibold text-foreground">{t.historyTitle}</h2>
            <ul className="mt-3 divide-y divide-border">
              {completed.map((attempt) => {
                const isCurrent = attempt.session_id === result.session_id;
                return (
                  <li key={attempt.session_id}>
                    <Link
                      href={`/student/tryout/${tryoutId}/result?attempt=${attempt.session_id}`}
                      aria-current={isCurrent ? "page" : undefined}
                      className={cn(
                        "flex items-center justify-between gap-3 rounded-lg px-2 py-3.5 transition-colors hover:bg-surface-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus",
                        isCurrent && "bg-accent-soft",
                      )}
                    >
                      <span>
                        <span className="block text-sm font-medium text-foreground">
                          {t.attemptLabel(attempt.attempt_number)}
                          {isCurrent && <span className="ml-2 text-xs font-normal text-muted">{t.currentAttempt}</span>}
                        </span>
                        {attempt.completed_at && <span className="block text-xs text-muted">{formatDate(attempt.completed_at)}</span>}
                      </span>
                      <span className="flex items-center gap-3">
                        <span className="font-display text-xl font-semibold text-foreground">{attempt.score ?? "–"}</span>
                        <ArrowRight className="size-4 text-muted" />
                      </span>
                    </Link>
                  </li>
                );
              })}
            </ul>
          </Card>
        )}

        <div className="flex flex-wrap gap-3">
          <LinkButton href={`/student/tryout/${tryoutId}`} variant="secondary" size="md">
            {t.historyAndRetry} <ArrowRight className="size-4" />
          </LinkButton>
          <LinkButton href="/student" variant="secondary" size="md">
            {t.backToDashboard}
          </LinkButton>
        </div>
      </div>
    </AppShell>
  );
}

function ResultRoute({ tryoutId }: { tryoutId: string }) {
  const attempt = useSearchParams().get("attempt");
  return <ResultContent key={attempt ?? "latest"} tryoutId={tryoutId} attemptParam={attempt} />;
}

export default function ResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ResultRoute tryoutId={id} />
    </Suspense>
  );
}
