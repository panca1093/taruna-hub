"use client";

import { Suspense, use, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, Chip } from "@heroui/react";
import { ArrowLeft, Clock, CloudOff, Lock } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LatexText } from "@/components/latex";
import { LinkButton } from "@/components/ui/link-button";
import { api, ApiError, type ReviewQuestion, type StudentTryout, type TryoutResultDetail, type TryoutReview } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useLocale } from "@/lib/i18n/locale-context";
import { useTryoutReviewT } from "@/lib/i18n/dict/tryout-review";

type LoadState = "loading" | "ready" | "failed" | "unavailable" | "notReady" | "held";

function ReviewContent({ tryoutId, attemptParam }: { tryoutId: string; attemptParam: string | null }) {
  const router = useRouter();
  const t = useTryoutReviewT();
  const { locale } = useLocale();
  const formatDate = (iso: string) =>
    new Intl.DateTimeFormat(locale === "id" ? "id-ID" : "en-US", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
      new Date(iso),
    );
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [result, setResult] = useState<TryoutResultDetail | null>(null);
  const [review, setReview] = useState<TryoutReview | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [subtestId, setSubtestId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const loaded = await api.runtime.overview(tryoutId);
      const sessionId = attemptParam ?? loaded.attempts.find((a) => a.status === "completed")?.session_id ?? null;
      if (!sessionId) return null;
      const res = await api.runtime.result(sessionId);
      if (res.tryout_id !== tryoutId) throw new ApiError(404, { code: "not_found", message: "" });
      // Keys and explanations are only requested once the server says they are released.
      if (!res.explanations_released) return { loaded, res, detail: null };
      try {
        return { loaded, res, detail: await api.runtime.review(sessionId) };
      } catch (err) {
        if (err instanceof ApiError && err.status === 409 && err.code === "explanations_not_released") {
          return { loaded, res, detail: null };
        }
        throw err;
      }
    };
    load()
      .then((data) => {
        if (cancelled) return;
        if (!data) {
          router.replace(`/student/tryout/${tryoutId}`);
          return;
        }
        setOverview(data.loaded);
        setResult(data.res);
        setReview(data.detail);
        setSubtestId(data.detail?.subtests[0]?.subtest_id ?? null);
        setState(data.detail ? "ready" : "held");
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

  const resultHref = `/student/tryout/${tryoutId}/result${attemptParam ? `?attempt=${attemptParam}` : ""}`;

  if (state === "loading") {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      </AppShell>
    );
  }
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
  if (state === "held" || !review || !result || !overview) {
    return (
      <AppShell title={t.pageTitle} description={overview?.title}>
        <EmptyState
          icon={Clock}
          title={t.heldTitle}
          description={`${t.heldBody}${result?.explanation_release_at ? ` ${t.heldUntil(formatDate(result.explanation_release_at))}` : ""}`}
          action={
            <LinkButton href={resultHref} variant="secondary" size="sm">
              {t.backToResult}
            </LinkButton>
          }
        />
      </AppShell>
    );
  }

  const active = review.subtests.find((st) => st.subtest_id === subtestId) ?? review.subtests[0];

  return (
    <AppShell title={t.pageTitle} description={overview.title}>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">{t.heading}</h1>
            <p className="mt-1 text-sm text-muted">{t.headingMeta(overview.title, result.attempt_number)}</p>
          </div>
          <LinkButton href={resultHref} variant="secondary" size="sm">
            <ArrowLeft className="size-3.5" /> {t.backToResult}
          </LinkButton>
        </div>

        <div role="tablist" aria-label={t.subtestsLabel} className="flex flex-wrap gap-2">
          {review.subtests.map((st) => {
            const selected = st.subtest_id === active?.subtest_id;
            return (
              <button
                key={st.subtest_id}
                type="button"
                role="tab"
                aria-selected={selected}
                onClick={() => setSubtestId(st.subtest_id)}
                className={cn(
                  "rounded-full border px-3.5 py-1.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus",
                  selected ? "border-accent bg-accent-soft text-accent-soft-foreground" : "border-border text-muted hover:text-foreground",
                )}
              >
                {st.name}
                <span className="ml-2 text-xs font-normal">{t.correctCount(st.correct_count, st.question_count)}</span>
              </button>
            );
          })}
        </div>

        {active && (
          <Card className="gap-0 p-6">
            <div className="divide-y divide-border">
              {active.questions.map((q, i) => (
                <ReviewItem key={q.id} q={q} index={i} subtestName={active.name} t={t} />
              ))}
            </div>
          </Card>
        )}
      </div>
    </AppShell>
  );
}

function ReviewItem({
  q,
  index,
  subtestName,
  t,
}: {
  q: ReviewQuestion;
  index: number;
  subtestName: string;
  t: ReturnType<typeof useTryoutReviewT>;
}) {
  const blank = q.answer.length === 0;
  const optionLabel = (value: string) => q.options.find((o) => o.id === value)?.label ?? value;
  const points = q.option_weighted && !blank ? q.options.find((o) => o.id === q.answer[0])?.points : null;
  const tone = q.is_correct
    ? "bg-success-soft text-success-soft-foreground"
    : blank
      ? "bg-default text-default-foreground"
      : "bg-danger-soft text-danger-soft-foreground";

  const list = (values: string[]) =>
    values.map((value, i) => (
      <span key={value}>
        {i > 0 && ", "}
        <LatexText text={optionLabel(value)} />
      </span>
    ));

  return (
    <article className="space-y-2.5 py-5 first:pt-0 last:pb-0">
      <Chip size="sm" className={cn("font-medium", tone)}>
        {subtestName} · {q.is_correct ? t.statusCorrect : blank ? t.statusBlank : t.statusWrong}
      </Chip>
      <div className="text-base font-medium text-foreground">
        <span className="text-muted">{index + 1}. </span>
        <LatexText text={q.content} />
      </div>
      <p className="text-sm text-muted">
        {t.yourAnswerPrefix}{" "}
        <strong className="font-semibold text-foreground">{blank ? t.emptyAnswer : list(q.answer)}</strong>
        {points != null && <span className="ml-1">{t.pointsSuffix(points)}</span>}
      </p>
      {!q.option_weighted && q.correct.length > 0 && (
        <p className="text-sm text-muted">
          {t.correctAnswerPrefix} <strong className="font-semibold text-foreground">{list(q.correct)}</strong>
        </p>
      )}
      <div className="rounded-lg border border-border bg-surface-secondary p-3 text-sm text-foreground">
        {q.explanation ? <LatexText text={q.explanation} /> : <span className="text-muted">{t.noExplanation}</span>}
      </div>
    </article>
  );
}

function ReviewRoute({ tryoutId }: { tryoutId: string }) {
  const attempt = useSearchParams().get("attempt");
  return <ReviewContent key={attempt ?? "latest"} tryoutId={tryoutId} attemptParam={attempt} />;
}

export default function ReviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ReviewRoute tryoutId={id} />
    </Suspense>
  );
}
