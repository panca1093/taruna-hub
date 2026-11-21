"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Icon } from "@/components/icon";
import { Panel, ScoreBar } from "@/components/page-blocks";
import type { TryoutRanking } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useTryoutResultT } from "@/lib/i18n/dict/tryout-result";
import { useLocale } from "@/lib/i18n/locale-context";

type ResultSubtest = { subtest_id: string; name: string; score: number; correct_count: number; question_count: number };
type HistoryAttempt = { session_id: string; attempt_number: number; score: number | null; completed_at: string | null };

const itemClass =
  "flex w-full items-center justify-between gap-4 rounded-control px-3 py-4 text-left text-caption outline-none transition-colors hover:text-accent focus-visible:ring-2 focus-visible:ring-focus";

/**
 * The score, rank, subtest and attempt-history blocks of one tryout attempt. `aside` fills the wide cell beside the
 * score card (the student's explanation panel); without it the score card spans one row. Attempts switch through
 * `attemptHref` (links) or `onSelectAttempt` (buttons).
 */
export function TryoutResultView({
  score,
  subtests,
  ranking,
  attempts,
  currentSessionId,
  attemptHref,
  onSelectAttempt,
  aside,
}: {
  score: number;
  subtests: ResultSubtest[];
  ranking: TryoutRanking;
  attempts: HistoryAttempt[];
  currentSessionId: string;
  attemptHref?: (sessionId: string) => string;
  onSelectAttempt?: (sessionId: string) => void;
  aside?: React.ReactNode;
}) {
  const t = useTryoutResultT();
  const { locale } = useLocale();
  const formatDate = (iso: string) =>
    new Intl.DateTimeFormat(locale === "id" ? "id-ID" : "en-US", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
      new Date(iso),
    );
  const correct = subtests.reduce((sum, s) => sum + s.correct_count, 0);
  const total = subtests.reduce((sum, s) => sum + s.question_count, 0);
  const stat = "rounded-tile border border-border bg-surface p-5 sm:p-5.5";

  return (
    <div className="space-y-6">
      <div className="grid gap-4 lg:grid-cols-4 lg:gap-6">
        <Panel
          as="section"
          tone="peach"
          radius="hero"
          className={cn("flex flex-col items-center justify-center text-center", aside && "lg:row-span-2")}
        >
          <p className="text-label text-foreground/80">{t.scoreLabel}</p>
          <p className="font-display my-5 text-score">{score}</p>
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
          <p className="font-display text-hero-lg">{t.rankValue(ranking.rank, ranking.participant_count)}</p>
          <p className="mt-1.5 text-micro text-muted">{t.statRank}</p>
          <p className="mt-1 text-micro text-muted">{t.rankNote(ranking.average_score, ranking.attempts_counted)}</p>
        </div>
        {aside}
      </div>

      <Panel as="section">
        <h2 className="font-display mb-5 text-heading-sm">{t.subtestScoresTitle}</h2>
        <div className="grid gap-5">
          {subtests.map((s) => (
            <ScoreBar
              key={s.subtest_id}
              label={s.name}
              valueLabel={`${s.score} / 100 · ${t.correctCount(s.correct_count, s.question_count)}`}
              value={s.score}
            />
          ))}
        </div>
      </Panel>

      {attempts.length > 0 && (
        <Panel as="section">
          <h2 className="font-display text-heading-sm">{t.historyTitle}</h2>
          <ul className="mt-2 divide-y divide-border">
            {attempts.map((attempt) => {
              const isCurrent = attempt.session_id === currentSessionId;
              const className = cn(itemClass, isCurrent && "bg-accent-soft");
              const content = (
                <>
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
                </>
              );
              return (
                <li key={attempt.session_id}>
                  {attemptHref ? (
                    <Link href={attemptHref(attempt.session_id)} aria-current={isCurrent ? "page" : undefined} className={className}>
                      {content}
                    </Link>
                  ) : (
                    <button type="button" aria-current={isCurrent ? "true" : undefined} className={className} onClick={() => onSelectAttempt?.(attempt.session_id)}>
                      {content}
                    </button>
                  )}
                </li>
              );
            })}
          </ul>
        </Panel>
      )}
    </div>
  );
}
