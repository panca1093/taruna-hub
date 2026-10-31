"use client";

import { Suspense, use, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@heroui/react";
import { ArrowRight, Clock, CloudOff, Lock } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { Icon } from "@/components/icon";
import { Panel, ScoreBar } from "@/components/page-blocks";
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
        <p className="py-14 text-center text-small text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const correct = result.subtests.reduce((sum, s) => sum + s.correct_count, 0);
  const total = result.subtests.reduce((sum, s) => sum + s.question_count, 0);
  const completed = overview.attempts.filter((a) => a.status === "completed");
  const reviewHref = `/student/tryout/${tryoutId}/review?attempt=${result.session_id}`;

  const stat = "rounded-tile border border-border bg-surface p-5 sm:p-5.5";

  return (
    <AppShell
      title={t.heading(result.attempt_number)}
      description={t.headingMeta(overview.title, formatDate(result.completed_at))}
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.crumbResult }]}
      actions={
        result.explanations_released ? (
          <LinkButton href={reviewHref} variant="primary" size="md">
            {t.viewReview} <Icon icon={ArrowRight} size="sm" />
          </LinkButton>
        ) : (
          <Button variant="secondary" isDisabled>
            <Icon icon={Clock} size="sm" /> {t.reviewHeld}
          </Button>
        )
      }
    >
      <div className="space-y-6">
        <div className="grid gap-4 lg:grid-cols-4 lg:gap-6">
          <Panel as="section" tone="peach" radius="hero" className="flex flex-col items-center justify-center text-center lg:row-span-2">
            <p className="text-label text-foreground/80">{t.scoreLabel}</p>
            <p className="font-display my-5 text-score">{result.score}</p>
            <p className="text-label text-foreground/80">{t.scoreOutOf}</p>
          </Panel>
          <div className={stat}>
            <p className="font-display text-hero-lg">{correct}</p>
            <p className="mt-1.5 text-micro text-muted">{t.statCorrect}</p>
          </div>
          <div className={stat}>
            <p className="font-display text-hero-lg">{total}</p>
            <p className="mt-1.5 text-micro text-muted">{t.statQuestions}</p>
          </div>
          <div className={stat}>
            <p className="font-display text-hero-lg">{t.rankValue(result.ranking.rank, result.ranking.participant_count)}</p>
            <p className="mt-1.5 text-micro text-muted">{t.statRank}</p>
            <p className="mt-1 text-micro text-muted">{t.rankNote(result.ranking.average_score, result.ranking.attempts_counted)}</p>
          </div>
          <Panel as="section" radius="card" className="lg:col-span-3">
            {result.explanations_released ? (
              <>
                <h2 className="font-display mb-3 text-heading-sm">{t.learnTitle}</h2>
                <p className="text-caption leading-relaxed text-muted">{t.learnBody}</p>
              </>
            ) : (
              <div className="flex items-start gap-3">
                <Icon icon={Clock} size="sm" className="mt-1 text-rose" />
                <div>
                  <h2 className="font-display mb-3 text-heading-sm">{t.heldTitle}</h2>
                  <p className="text-caption leading-relaxed text-muted">{t.heldBody}</p>
                  {result.explanation_release_at && (
                    <p className="mt-1 text-caption font-medium">{t.heldUntil(formatDate(result.explanation_release_at))}</p>
                  )}
                </div>
              </div>
            )}
          </Panel>
        </div>

        <Panel as="section">
          <h2 className="font-display mb-5 text-heading-sm">{t.subtestScoresTitle}</h2>
          <div className="grid gap-5">
            {result.subtests.map((s) => (
              <ScoreBar
                key={s.subtest_id}
                label={s.name}
                valueLabel={`${s.score} / 100 · ${t.correctCount(s.correct_count, s.question_count)}`}
                value={s.score}
              />
            ))}
          </div>
        </Panel>

        {completed.length > 0 && (
          <Panel as="section">
            <h2 className="font-display text-heading-sm">{t.historyTitle}</h2>
            <ul className="mt-2 divide-y divide-border">
              {completed.map((attempt) => {
                const isCurrent = attempt.session_id === result.session_id;
                return (
                  <li key={attempt.session_id}>
                    <Link
                      href={`/student/tryout/${tryoutId}/result?attempt=${attempt.session_id}`}
                      aria-current={isCurrent ? "page" : undefined}
                      className={cn(
                        "flex items-center justify-between gap-4 rounded-control px-3 py-4 text-caption outline-none transition-colors hover:text-accent focus-visible:ring-2 focus-visible:ring-focus",
                        isCurrent && "bg-accent-soft",
                      )}
                    >
                      <span>
                        <span className="block">
                          {t.attemptLabel(attempt.attempt_number)}
                          {isCurrent && <span className="ml-2 text-micro text-muted">{t.currentAttempt}</span>}
                        </span>
                        {attempt.completed_at && <span className="mt-1 block text-micro text-muted">{formatDate(attempt.completed_at)}</span>}
                      </span>
                      <span className="flex items-center gap-5">
                        <span className="font-display text-heading">{attempt.score ?? "–"}</span>
                        <Icon icon={ArrowRight} size="sm" />
                      </span>
                    </Link>
                  </li>
                );
              })}
            </ul>
          </Panel>
        )}

        <div className="flex flex-wrap items-center gap-3.5">
          <LinkButton href={`/student/tryout/${tryoutId}`} variant="secondary" size="md">
            {t.historyAndRetry} <Icon icon={ArrowRight} size="sm" />
          </LinkButton>
          <LinkButton href="/student" variant="secondary" size="md">
            {t.backToDashboard} <Icon icon={ArrowRight} size="sm" />
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
