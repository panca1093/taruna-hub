"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ProgressLineChart } from "@/components/charts";
import { Panel, ScoreBar } from "@/components/page-blocks";
import type { TryoutRanking } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useTryoutResultT } from "@/lib/i18n/dict/tryout-result";
import { useLocale } from "@/lib/i18n/locale-context";

type ResultSubtest = { subtest_id: string; name: string; score: number; correct_count: number; question_count: number };
type HistoryAttempt = {
  session_id: string;
  attempt_number: number;
  score: number | null;
  completed_at: string | null;
  correct_count: number | null;
  wrong_count: number | null;
  blank_count: number | null;
};

const RECENT = 5;

const attemptLinkClass = "rounded-chip font-medium outline-none hover:text-accent focus-visible:ring-2 focus-visible:ring-focus";

/**
 * One tryout attempt in the context of the student's progress: the average over all attempts beside the score-per-attempt
 * chart, this attempt's tiles and subtest scores, then the correct/wrong table of the last five attempts. `aside` is
 * the student's explanation panel. Attempts switch through `attemptHref` (links) or `onSelectAttempt` (buttons).
 */
export function TryoutResultView({
  subtests,
  ranking,
  attempts,
  currentSessionId,
  attemptHref,
  onSelectAttempt,
  aside,
}: {
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
  const router = useRouter();
  const scored = attempts.filter((attempt) => attempt.score !== null).sort((a, b) => a.attempt_number - b.attempt_number);

  const recent = [...scored].reverse().slice(0, RECENT);
  const select = (sessionId: string) => (attemptHref ? router.push(attemptHref(sessionId)) : onSelectAttempt?.(sessionId));
  const cell = "border-b border-border px-2.5 py-3";

  return (
    <div className="space-y-6">
      <div className="grid gap-5 sm:gap-7.5 md:grid-cols-score">
        <Panel as="section" tone="peach" radius="hero" className="flex flex-col items-center justify-center text-center">
          <p className="text-label text-foreground/80">{t.averageLabel(ranking.attempts_counted)}</p>
          <p className="font-display my-5 text-score tabular-nums">{ranking.average_score}</p>
          <p className="text-label text-foreground/80">{t.scoreOutOf}</p>
          <p className="mt-5 rounded-full bg-surface/60 px-3 py-1 text-label font-medium">{t.rankLine(ranking.rank, ranking.participant_count)}</p>
        </Panel>
        <Panel as="section">
          <h2 className="font-display text-heading-sm">{t.progressTitle}</h2>
          <ProgressLineChart
            className="mt-4"
            points={scored.map((attempt) => ({ key: attempt.session_id, label: String(attempt.attempt_number), value: attempt.score ?? 0, counted: true }))}
            selectedKey={currentSessionId}
            onSelect={select}
            pointLabel={(point) => t.progressPoint(Number(point.label), point.value)}
          />
        </Panel>
      </div>

      {aside}

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

      {recent.length > 0 && (
        <Panel as="section">
          <h2 className="font-display text-heading-sm">{t.recentTitle(recent.length)}</h2>
          <div className="mt-2 overflow-x-auto">
            <table className="w-full min-w-96 border-collapse text-caption">
              <thead>
                <tr className="text-micro font-medium text-muted">
                  <th className={cn(cell, "text-left font-medium")}>{t.colAttempt}</th>
                  <th className={cn(cell, "text-right font-medium")}>{t.colCorrect}</th>
                  <th className={cn(cell, "text-right font-medium")}>{t.colWrong}</th>
                  <th className={cn(cell, "text-right font-medium")}>{t.colBlank}</th>
                  <th className={cn(cell, "text-right font-medium")}>{t.colScore}</th>
                </tr>
              </thead>
              <tbody className="tabular-nums">
                {recent.map((attempt) => {
                  const isCurrent = attempt.session_id === currentSessionId;
                  const label = t.attemptLabel(attempt.attempt_number);
                  return (
                    <tr key={attempt.session_id} className={cn(isCurrent && "bg-accent-soft")}>
                      <td className={cn(cell, "text-left")}>
                        {attemptHref ? (
                          <Link href={attemptHref(attempt.session_id)} aria-current={isCurrent ? "page" : undefined} className={attemptLinkClass}>
                            {label}
                          </Link>
                        ) : onSelectAttempt ? (
                          <button type="button" aria-current={isCurrent ? "true" : undefined} className={attemptLinkClass} onClick={() => onSelectAttempt(attempt.session_id)}>
                            {label}
                          </button>
                        ) : (
                          <span className="font-medium">{label}</span>
                        )}
                        {attempt.completed_at && <span className="mt-0.5 block text-micro text-muted">{formatDate(attempt.completed_at)}</span>}
                      </td>
                      <td className={cn(cell, "text-right")}>{attempt.correct_count ?? "–"}</td>
                      <td className={cn(cell, "text-right")}>{attempt.wrong_count ?? "–"}</td>
                      <td className={cn(cell, "text-right")}>{attempt.blank_count ?? "–"}</td>
                      <td className={cn(cell, "text-right font-semibold")}>{attempt.score}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
    </div>
  );
}
