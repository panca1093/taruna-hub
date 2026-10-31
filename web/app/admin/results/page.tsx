"use client";

import { useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { AlertTriangle, BarChart3, CloudOff, Trophy, TriangleAlert } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { StatusChip } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput } from "@/components/ui/form";
import { api, errorText, type Leaderboard, type StaffResult, type TryoutSummary, type ViolationTimeline } from "@/lib/api/client";
import { useResultsT } from "@/lib/i18n/dict/results";

const PAGE_SIZE = 50;
const BOARD_SIZE = 20;

const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

export default function ResultsPage() {
  const t = useResultsT();
  const [tryouts, setTryouts] = useState<TryoutSummary[]>([]);
  const [tryoutFilter, setTryoutFilter] = useState("all");
  const [rows, setRows] = useState<StaffResult[] | null>(null);
  const [total, setTotal] = useState(0);
  const [board, setBoard] = useState<Leaderboard | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [detail, setDetail] = useState<StaffResult | null>(null);
  const [violations, setViolations] = useState<ViolationTimeline | null>(null);
  const [violationsFailed, setViolationsFailed] = useState(false);

  const openDetail = (row: StaffResult) => {
    setViolations(null);
    setViolationsFailed(false);
    setDetail(row);
  };

  useEffect(() => {
    if (!detail) return;
    let cancelled = false;
    api.monitoring
      .violations(detail.session_id)
      .then((loaded) => {
        if (!cancelled) setViolations(loaded);
      })
      .catch(() => {
        if (!cancelled) setViolationsFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [detail]);

  useEffect(() => {
    let cancelled = false;
    api.tryouts
      .list({ limit: 100 })
      .then((page) => {
        if (!cancelled) setTryouts(page.tryouts);
      })
      .catch(() => {
        // The filter is a convenience; the list below still loads and reports its own failure.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const tryoutId = tryoutFilter === "all" ? undefined : tryoutFilter;
    api.results
      .list({ tryout_id: tryoutId, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.results);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    const boardRequest = tryoutId ? api.results.leaderboard(tryoutId, BOARD_SIZE) : Promise.resolve(null);
    boardRequest
      .then((loaded) => {
        if (!cancelled) setBoard(loaded);
      })
      .catch(() => {
        if (!cancelled) setBoard(null);
      });
    return () => {
      cancelled = true;
    };
  }, [tryoutFilter, reloadKey]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const changeTryout = (next: string) => {
    setRows(null);
    setBoard(null);
    setLoadFailed(false);
    setTryoutFilter(next);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const page = await api.results.list({
        tryout_id: tryoutFilter === "all" ? undefined : tryoutFilter,
        limit: PAGE_SIZE,
        offset: rows?.length ?? 0,
      });
      setRows((prev) => {
        const seen = new Set((prev ?? []).map((row) => row.session_id));
        return [...(prev ?? []), ...page.results.filter((row) => !seen.has(row.session_id))];
      });
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const tryoutOptions = [{ id: "all", label: t.allTryouts }, ...tryouts.map((tr) => ({ id: tr.id, label: tr.title }))];

  const columns: Column<StaffResult>[] = [
    {
      key: "student",
      header: t.colStudent,
      render: (r) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={{ name: r.student_name }} size="sm" />
          <span className="font-medium text-foreground">{r.student_name}</span>
        </div>
      ),
    },
    {
      key: "tryout",
      header: t.colTryout,
      render: (r) => (
        <div>
          <p className="text-foreground">{r.tryout_title}</p>
          <p className="text-label text-muted">{r.program_code}</p>
        </div>
      ),
    },
    { key: "attempt", header: t.colAttempt, render: (r) => <span className="text-muted">#{r.attempt_number}</span> },
    { key: "score", header: t.colScore, render: (r) => <span className="font-mono font-semibold text-foreground">{r.score}</span> },
    { key: "rank", header: t.colRank, render: (r) => `${r.rank} / ${r.participant_count}` },
    {
      key: "irt",
      header: t.colIrt,
      render: (r) =>
        r.irt_warning ? (
          <StatusChip tone="warning" className="gap-1">
            <Icon icon={TriangleAlert} size="xs" /> {t.irtWarning}
          </StatusChip>
        ) : (
          <span className="text-muted">—</span>
        ),
    },
    { key: "completedAt", header: t.colCompleted, render: (r) => formatDateTime(r.completed_at) },
  ];

  return (
    <AppShell title={t.pageTitle} description={rows === null ? undefined : t.resultCount(total)}>
      <div className="mb-4 max-w-xs">
        <SelectInput options={tryoutOptions} selectedKey={tryoutFilter} onSelectionChange={(k) => changeTryout(k ?? "all")} />
      </div>

      {board && board.entries.length > 0 && (
        <div className="mb-6">
          <h2 className="mb-3 flex items-center gap-2 font-display text-heading-sm">
            <Icon icon={Trophy} size="sm" className="text-rose" /> {t.leaderboardTitle}
          </h2>
          <div className="space-y-2">
            {board.entries.map((row) => (
              <div key={row.user_id} className="flex items-center gap-3 rounded-tile border border-border bg-surface px-4 py-3 text-small">
                <span className="w-6 shrink-0 text-center font-mono text-label font-semibold text-muted">{row.rank}</span>
                <GradientAvatar user={{ name: row.student_name }} size="sm" />
                <span className="min-w-0 flex-1 truncate font-medium text-foreground">{row.student_name}</span>
                <span className="text-label text-muted">{t.attemptCount(row.attempts_counted)}</span>
                <span className="font-mono font-semibold text-foreground">{row.average_score}</span>
              </div>
            ))}
          </div>
          {board.total > board.entries.length && (
            <p className="mt-2 text-label text-muted">{t.leaderboardMore(board.entries.length, board.total)}</p>
          )}
        </div>
      )}

      <h2 className="mb-3 font-display text-heading-sm">{t.historyTitle}</h2>
      {loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" size="sm" onPress={retry}>
              {t.retry}
            </Button>
          }
        />
      ) : rows === null ? (
        <p className="py-14 text-center text-caption text-muted">{t.loading}</p>
      ) : rows.length === 0 ? (
        <EmptyState icon={BarChart3} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <>
          <DataTable columns={columns} rows={rows} rowKey={(r) => r.session_id} onRowClick={openDetail} />
          {rows.length < total && (
            <div className="mt-4 flex justify-center">
              <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                {loadingMore ? t.loadingMore : t.loadMore}
              </Button>
            </div>
          )}
        </>
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detail?.student_name ?? ""}
        description={detail?.tryout_title}
        size="lg"
        footer={
          <Button variant="ghost" onPress={() => setDetail(null)}>
            {t.close}
          </Button>
        }
      >
        {detail && (
          <>
            <div className="flex items-center gap-3">
              <GradientAvatar user={{ name: detail.student_name }} size="lg" />
              <div className="flex-1">
                <p className="font-display text-title text-foreground">{detail.student_name}</p>
                <p className="text-label text-muted">{detail.tryout_title}</p>
              </div>
              {detail.irt_warning && (
                <StatusChip tone="warning" className="gap-1">
                  <Icon icon={TriangleAlert} size="xs" /> {t.irtWarningBadge}
                </StatusChip>
              )}
            </div>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-4 rounded-tile border border-border p-5 text-small sm:grid-cols-4">
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailTotalScore}</dt>
                <dd className="font-mono text-title font-semibold text-foreground">{detail.score} / 100</dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailAverage}</dt>
                <dd className="font-mono text-title font-semibold text-foreground">{detail.average_score}</dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailRank}</dt>
                <dd className="font-medium text-foreground">
                  {detail.rank} / {detail.participant_count}
                </dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailCompleted}</dt>
                <dd className="font-medium text-foreground">{formatDateTime(detail.completed_at)}</dd>
              </div>
            </dl>
            <div>
              <h3 className="mb-3 font-display text-title">{t.subtestBreakdownTitle}</h3>
              <ul className="space-y-2">
                {detail.subtests.map((s) => (
                  <li
                    key={s.subtest_id}
                    className="flex items-center justify-between gap-3 rounded-control border border-border px-4 py-3 text-small"
                  >
                    <span className="font-medium text-foreground">{s.name}</span>
                    <span className="flex items-center gap-3 text-label text-muted">
                      <span>{t.correctOf(s.correct_count, s.question_count)}</span>
                      <span className="font-mono font-semibold text-foreground">{s.score}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </div>
            <div>
              {violationsFailed ? (
                <p className="text-caption text-danger">{t.violationsError}</p>
              ) : violations === null ? (
                <p className="text-caption text-muted">{t.loading}</p>
              ) : (
                <>
                  <h3 className="mb-3 font-display text-title">{t.violationHistoryTitle(violations.total)}</h3>
                  {violations.violations.length === 0 ? (
                    <p className="text-caption text-muted">{t.noViolations}</p>
                  ) : (
                    <ul>
                      {violations.violations.map((v) => (
                        <li key={v.id} className="relative flex items-baseline gap-3 py-3 not-last:after:absolute not-last:after:top-7 not-last:after:-bottom-3 not-last:after:left-1 not-last:after:w-px not-last:after:bg-border">
                          <span className="z-10 size-2 shrink-0 rounded-full bg-danger" aria-hidden />
                          <div className="flex min-w-0 flex-1 flex-wrap items-baseline justify-between gap-x-3">
                            <span className="flex items-center gap-2 text-small font-medium text-foreground">
                              <Icon icon={AlertTriangle} size="xs" className="text-danger" />
                              {v.type === "screenshot_attempt" ? t.violationScreenshot : t.violationFullscreenExit}
                            </span>
                            <time className="text-label text-muted">{formatDateTime(v.occurred_at)}</time>
                          </div>
                        </li>
                      ))}
                    </ul>
                  )}
                </>
              )}
            </div>
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
