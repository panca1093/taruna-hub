"use client";

import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Accordion, Button, Card, Chip } from "@heroui/react";
import { CheckCircle2, ChevronDown, Clock, CloudOff, Trophy, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { EmptyState } from "@/components/empty-state";
import { LatexText } from "@/components/latex";
import { LinkButton } from "@/components/ui/link-button";
import { api, ApiError, type ReviewQuestion, type StudentTryout, type TryoutResultDetail, type TryoutReview } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useTryoutResultT } from "@/lib/i18n/dict/tryout-result";

export default function ResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const t = useTryoutResultT();
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [result, setResult] = useState<TryoutResultDetail | null>(null);
  const [review, setReview] = useState<TryoutReview | null>(null);
  const [reviewFailed, setReviewFailed] = useState(false);
  const [failed, setFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.runtime
      .overview(id)
      .then(async (loaded) => {
        const latest = loaded.attempts.find((a) => a.status === "completed");
        if (!latest) {
          if (!cancelled) router.replace(`/student/tryout/${id}`);
          return;
        }
        const res = await api.runtime.result(latest.session_id);
        if (cancelled) return;
        setOverview(loaded);
        setResult(res);
        if (!res.explanations_released) return;
        try {
          const loadedReview = await api.runtime.review(latest.session_id);
          if (!cancelled) setReview(loadedReview);
        } catch {
          if (!cancelled) setReviewFailed(true);
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) router.replace(`/student/tryout/${id}`);
        else setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [id, router, reloadKey]);

  if (failed) {
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
                setFailed(false);
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
  if (!result || !overview) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  return (
    <AppShell title={t.pageTitle} description={overview.title}>
      <div className="mx-auto max-w-3xl space-y-6">
        <Card className="flex-col items-center gap-4 p-8 text-center sm:flex-row sm:text-left">
          <CountdownRing value={result.score} size={110} strokeWidth={9} animate>
            <div className="text-center">
              <p className="font-display text-2xl font-semibold text-foreground">{result.score}</p>
              <p className="text-[10px] text-muted">/ 100</p>
            </div>
          </CountdownRing>
          <div className="flex-1">
            <p className="text-sm font-medium text-muted">{t.yourTotalScore}</p>
            <p className="font-display mt-0.5 text-xl font-semibold text-foreground">{overview.title}</p>
            <div className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent-soft-foreground">
              <Trophy className="size-3.5" />
              {t.rankLabel(result.ranking.rank, result.ranking.participant_count)}
            </div>
          </div>
        </Card>

        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.subtestScoresTitle}</p>
          <div className="grid gap-3 sm:grid-cols-3">
            {result.subtests.map((s) => (
              <Card key={s.subtest_id} className="gap-2 p-4">
                <p className="line-clamp-1 text-xs font-medium text-muted">{s.name}</p>
                <p className="font-display text-xl font-semibold text-foreground">{s.score}</p>
                <p className="text-xs text-muted">{t.correctCount(s.correct_count, s.question_count)}</p>
              </Card>
            ))}
          </div>
        </div>

        <div className="flex items-center justify-between">
          <p className="text-sm font-semibold text-foreground">{t.attemptHistoryTitle}</p>
          <LinkButton href={`/student/tryout/${id}`} variant="ghost" size="sm">
            {t.viewAllAttempts}
          </LinkButton>
        </div>

        {!result.explanations_released ? (
          <div className="flex items-start gap-3 rounded-xl border border-border bg-surface-secondary p-4 text-sm text-muted">
            <Clock className="mt-0.5 size-4 shrink-0" />
            <p>
              {t.explanationWithheldPrefix}{" "}
              {result.explanation_release_at && (
                <strong className="text-foreground">{formatDate(result.explanation_release_at)}</strong>
              )}
              . {t.explanationWithheldSuffix}
            </p>
          </div>
        ) : reviewFailed ? (
          <p className="text-sm text-muted">{t.reviewErrorTitle}</p>
        ) : !review ? (
          <p className="text-sm text-muted">{t.loadingReview}</p>
        ) : (
          <div>
            <p className="mb-3 text-sm font-semibold text-foreground">{t.explanationsTitle}</p>
            <Accordion className="flex flex-col gap-3" allowsMultipleExpanded>
              {review.subtests.map((st) => (
                <Accordion.Item key={st.subtest_id} id={st.subtest_id} className="rounded-xl border border-border bg-surface px-4">
                  <Accordion.Heading>
                    <Accordion.Trigger className="flex w-full items-center justify-between gap-3 py-3.5 text-left">
                      <span className="text-sm font-medium text-foreground">{st.name}</span>
                      <span className="flex items-center gap-2 text-xs text-muted">
                        {t.correctCount(st.correct_count, st.question_count)}
                        <Accordion.Indicator>
                          <ChevronDown className="size-4" />
                        </Accordion.Indicator>
                      </span>
                    </Accordion.Trigger>
                  </Accordion.Heading>
                  <Accordion.Panel>
                    <Accordion.Body className="flex flex-col gap-4 border-t border-border pb-4 pt-4">
                      {st.questions.map((q, i) => (
                        <ReviewItem key={q.id} q={q} index={i} t={t} />
                      ))}
                    </Accordion.Body>
                  </Accordion.Panel>
                </Accordion.Item>
              ))}
            </Accordion>
          </div>
        )}
      </div>
    </AppShell>
  );
}

const formatDate = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

function ReviewItem({ q, index, t }: { q: ReviewQuestion; index: number; t: ReturnType<typeof useTryoutResultT> }) {
  const labelOf = (value: string) => q.options.find((o) => o.id === value)?.label ?? value;
  const pointsOf = (value: string) => q.options.find((o) => o.id === value)?.points;
  const weightedPoints = q.option_weighted && q.answer.length ? pointsOf(q.answer[0]) : null;
  return (
    <div className="space-y-2 border-b border-border pb-4 last:border-b-0 last:pb-0">
      <div className="flex items-start gap-2.5">
        <span
          className={cn(
            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full",
            q.is_correct ? "bg-success-soft text-success-soft-foreground" : "bg-danger-soft text-danger-soft-foreground",
          )}
        >
          {q.is_correct ? <CheckCircle2 className="size-3.5" /> : <XCircle className="size-3.5" />}
        </span>
        <div className="min-w-0 flex-1 text-sm text-foreground">
          <span className="text-muted">{index + 1}. </span>
          <LatexText text={q.content} />
        </div>
      </div>
      <div className="ml-7.5 flex flex-wrap gap-2 text-xs">
        <Chip
          size="sm"
          className={cn(q.is_correct ? "bg-success-soft text-success-soft-foreground" : "bg-danger-soft text-danger-soft-foreground", "font-medium")}
        >
          {t.yourAnswerPrefix} {q.answer.length ? q.answer.map(labelOf).join(", ") : t.emptyAnswer}
          {weightedPoints != null ? ` ${t.pointsSuffix(weightedPoints)}` : ""}
        </Chip>
        {!q.is_correct && !q.option_weighted && q.correct.length > 0 && (
          <Chip size="sm" className="bg-success-soft text-success-soft-foreground font-medium">
            {t.correctAnswerPrefix} {q.correct.map(labelOf).join(", ")}
          </Chip>
        )}
      </div>
      <p className="ml-7.5 text-xs leading-relaxed text-muted">
        <LatexText text={q.explanation} />
      </p>
    </div>
  );
}
