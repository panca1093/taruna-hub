"use client";

import { use, useEffect } from "react";
import { notFound, useRouter } from "next/navigation";
import { Accordion, Card, Chip } from "@heroui/react";
import { CheckCircle2, ChevronDown, Clock, Info, Trophy, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { LatexText } from "@/components/latex";
import { LinkButton } from "@/components/ui/link-button";
import { useUser } from "@/lib/session-context";
import { computeResult, estimateRank, getAttempts, loadSession } from "@/lib/tryout-session";
import { formatDate, getQuestionsForSubtest, getSubtest, getTryout } from "@/lib/mock-data";
import { cn } from "@/lib/cn";
import { useTryoutResultT } from "@/lib/i18n/dict/tryout-result";
import { useHydrated } from "@/lib/use-hydrated";

export default function ResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const user = useUser();
  const tryout = getTryout(id);
  const t = useTryoutResultT();
  const hydrated = useHydrated();
  const loaded = hydrated && tryout ? loadSession(tryout.id, user.id) : null;
  const session = loaded?.status === "completed" ? loaded : null;
  const answers: Record<string, string[]> = session?.answers ?? {};
  let result = null;
  if (tryout && session) {
    // Ranking is based on the average score across all of the student's own
    // attempts on this tryout (FR-RES-05), not just this attempt's score.
    const attempts = getAttempts(tryout.id, user.id);
    const attemptScores = attempts.map((a) => computeResult(tryout, a, { rank: 1, participantCount: 1 }).totalScore);
    const avgScore = Math.round(attemptScores.reduce((sum, s) => sum + s, 0) / attemptScores.length);
    result = computeResult(tryout, session, estimateRank(tryout.id, avgScore));
  }
  const mustRedirect = hydrated && !!tryout && !session;

  useEffect(() => {
    if (mustRedirect) router.replace(`/student/tryout/${id}`);
  }, [mustRedirect, router, id]);

  if (!tryout) notFound();
  if (!result) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  return (
    <AppShell title={t.pageTitle} description={tryout.title}>
      <div className="mx-auto max-w-3xl space-y-6">
        {result.irtWarning && (
          <div className="flex items-start gap-3 rounded-xl border border-warning/30 bg-warning-soft p-4 text-sm text-warning-soft-foreground">
            <Info className="mt-0.5 size-4 shrink-0" />
            <p>
              {t.irtWarning}
            </p>
          </div>
        )}

        <Card className="flex-col items-center gap-4 p-8 text-center sm:flex-row sm:text-left">
          <CountdownRing value={result.totalScore} size={110} strokeWidth={9} animate>
            <div className="text-center">
              <p className="font-display text-2xl font-semibold text-foreground">{result.totalScore}</p>
              <p className="text-[10px] text-muted">/ 100</p>
            </div>
          </CountdownRing>
          <div className="flex-1">
            <p className="text-sm font-medium text-muted">{t.yourTotalScore}</p>
            <p className="font-display mt-0.5 text-xl font-semibold text-foreground">{tryout.title}</p>
            <div className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent-soft-foreground">
              <Trophy className="size-3.5" />
              {t.rankLabel(result.rank, result.participantCount)}
            </div>
          </div>
        </Card>

        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.subtestScoresTitle}</p>
          <div className="grid gap-3 sm:grid-cols-3">
            {result.subtestScores.map((s) => {
              const subtest = getSubtest(s.subtestId);
              return (
                <Card key={s.subtestId} className="gap-2 p-4">
                  <p className="line-clamp-1 text-xs font-medium text-muted">{subtest?.name}</p>
                  <p className="font-display text-xl font-semibold text-foreground">{s.score}</p>
                  <p className="text-xs text-muted">
                    {t.correctCount(s.correct, s.total)}
                  </p>
                </Card>
              );
            })}
          </div>
        </div>

        <div className="flex items-center justify-between">
          <p className="text-sm font-semibold text-foreground">{t.attemptHistoryTitle}</p>
          <LinkButton href={`/student/tryout/${tryout.id}`} variant="ghost" size="sm">
            {t.viewAllAttempts}
          </LinkButton>
        </div>

        {tryout.explanationReleaseAt && new Date() < new Date(tryout.explanationReleaseAt) ? (
          <div className="flex items-start gap-3 rounded-xl border border-border bg-surface-secondary p-4 text-sm text-muted">
            <Clock className="mt-0.5 size-4 shrink-0" />
            <p>
              {t.explanationWithheldPrefix}{" "}
              <strong className="text-foreground">{formatDate(tryout.explanationReleaseAt)}</strong>. {t.explanationWithheldSuffix}
            </p>
          </div>
        ) : (
        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.explanationsTitle}</p>
          <Accordion className="flex flex-col gap-3" allowsMultipleExpanded>
            {result.subtestScores.map((s) => {
              const subtest = getSubtest(s.subtestId);
              if (!subtest) return null;
              const questions = getQuestionsForSubtest(subtest.id);
              return (
                <Accordion.Item key={subtest.id} id={subtest.id} className="rounded-xl border border-border bg-surface px-4">
                  <Accordion.Heading>
                    <Accordion.Trigger className="flex w-full items-center justify-between gap-3 py-3.5 text-left">
                      <span className="text-sm font-medium text-foreground">{subtest.name}</span>
                      <span className="flex items-center gap-2 text-xs text-muted">
                        {t.correctCount(s.correct, s.total)}
                        <Accordion.Indicator>
                          <ChevronDown className="size-4" />
                        </Accordion.Indicator>
                      </span>
                    </Accordion.Trigger>
                  </Accordion.Heading>
                  <Accordion.Panel>
                    <Accordion.Body className="flex flex-col gap-4 border-t border-border pb-4 pt-4">
                      {questions.map((q, i) => {
                        const given = answers[q.id] ?? [];
                        const optionLabel = (id: string) => q.options?.find((o) => o.id === id)?.label ?? id;
                        const pointsOf = (id: string) => q.options?.find((o) => o.id === id)?.points ?? 1;
                        const isCorrect = q.optionWeighted
                          ? given.length > 0
                          : q.type === "isian"
                            ? q.correctAnswer.some((a) => a.trim().toLowerCase() === (given[0] ?? "").trim().toLowerCase())
                            : [...q.correctAnswer].sort().join(",") === [...given].sort().join(",");

                        return (
                          <div key={q.id} className="space-y-2 border-b border-border pb-4 last:border-b-0 last:pb-0">
                            <div className="flex items-start gap-2.5">
                              <span
                                className={cn(
                                  "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full",
                                  isCorrect ? "bg-success-soft text-success-soft-foreground" : "bg-danger-soft text-danger-soft-foreground",
                                )}
                              >
                                {isCorrect ? <CheckCircle2 className="size-3.5" /> : <XCircle className="size-3.5" />}
                              </span>
                              <div className="min-w-0 flex-1 text-sm text-foreground">
                                <span className="text-muted">{i + 1}. </span>
                                <LatexText text={q.content} />
                              </div>
                            </div>
                            <div className="ml-7.5 flex flex-wrap gap-2 text-xs">
                              <Chip size="sm" className={cn(isCorrect ? "bg-success-soft text-success-soft-foreground" : "bg-danger-soft text-danger-soft-foreground", "font-medium")}>
                                {t.yourAnswerPrefix} {given.length ? given.map(optionLabel).join(", ") : t.emptyAnswer}
                                {q.optionWeighted && given.length ? ` ${t.pointsSuffix(pointsOf(given[0]))}` : ""}
                              </Chip>
                              {!isCorrect && !q.optionWeighted && (
                                <Chip size="sm" className="bg-success-soft text-success-soft-foreground font-medium">
                                  {t.correctAnswerPrefix} {q.correctAnswer.map(optionLabel).join(", ")}
                                </Chip>
                              )}
                            </div>
                            <p className="ml-7.5 text-xs leading-relaxed text-muted">
                              <LatexText text={q.explanation} />
                            </p>
                          </div>
                        );
                      })}
                    </Accordion.Body>
                  </Accordion.Panel>
                </Accordion.Item>
              );
            })}
          </Accordion>
        </div>
        )}
      </div>
    </AppShell>
  );
}
