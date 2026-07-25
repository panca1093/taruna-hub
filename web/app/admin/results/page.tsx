"use client";

import { useMemo, useState } from "react";
import { Button, Chip } from "@heroui/react";
import { AlertTriangle, BarChart3, Download, Trophy, TriangleAlert } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput } from "@/components/ui/form";
import { tryoutResults, findUser, formatDateTime, getLeaderboardForTryout, getProgram, getSubtest, getTryout, tryouts, violations } from "@/lib/mock-data";
import { useResultsT } from "@/lib/i18n/dict/results";
import type { TryoutResult } from "@/lib/types";

function downloadCsv(rows: TryoutResult[], t: ReturnType<typeof useResultsT>) {
  const lines = rows.map((r) =>
    [
      getTryout(r.tryoutId)?.title ?? r.tryoutId,
      findUser(r.userId)?.name ?? r.userId,
      r.totalScore,
      r.rank,
      r.participantCount,
      r.irtWarning ? t.csvYes : t.csvNo,
      r.completedAt,
    ]
      .map((v) => `"${String(v).replace(/"/g, '""')}"`)
      .join(","),
  );
  const csv = [t.csvHeaders.join(","), ...lines].join("\n");
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "hasil-tryout.csv";
  a.click();
  URL.revokeObjectURL(url);
}

export default function ResultsPage() {
  const t = useResultsT();
  const tryoutOptions = [{ id: "all", label: t.allTryouts }, ...tryouts.map((tr) => ({ id: tr.id, label: tr.title }))];
  const [tryoutFilter, setTryoutFilter] = useState("all");
  const [detail, setDetail] = useState<TryoutResult | null>(null);

  const filtered = useMemo(
    () => (tryoutFilter === "all" ? tryoutResults : tryoutResults.filter((r) => r.tryoutId === tryoutFilter)).sort((a, b) => a.rank - b.rank),
    [tryoutFilter],
  );

  const columns: Column<TryoutResult>[] = [
    {
      key: "student",
      header: t.colStudent,
      render: (r) => {
        const u = findUser(r.userId);
        return u ? (
          <div className="flex items-center gap-2.5">
            <GradientAvatar user={u} size="sm" />
            <span className="font-medium text-foreground">{u.name}</span>
          </div>
        ) : (
          r.userId
        );
      },
    },
    {
      key: "tryout",
      header: t.colTryout,
      render: (r) => {
        const tr = getTryout(r.tryoutId);
        const p = tr ? getProgram(tr.programId) : undefined;
        return (
          <div>
            <p className="text-foreground">{tr?.title}</p>
            {p && <p className="text-xs text-muted">{p.code}</p>}
          </div>
        );
      },
    },
    { key: "attempt", header: t.colAttempt, render: (r) => <span className="text-muted">#{r.attemptNumber}</span> },
    { key: "score", header: t.colScore, render: (r) => <span className="font-mono font-semibold text-foreground">{r.totalScore}</span> },
    { key: "rank", header: t.colRank, render: (r) => `${r.rank} / ${r.participantCount}` },
    {
      key: "irt",
      header: t.colIrt,
      render: (r) =>
        r.irtWarning ? (
          <Chip size="sm" className="bg-warning-soft text-warning-soft-foreground gap-1 font-medium">
            <TriangleAlert className="size-3" /> {t.irtWarning}
          </Chip>
        ) : (
          <span className="text-muted">—</span>
        ),
    },
    { key: "completedAt", header: t.colCompleted, render: (r) => formatDateTime(r.completedAt) },
  ];

  const detailTryout = detail ? getTryout(detail.tryoutId) : undefined;
  const detailUser = detail ? findUser(detail.userId) : undefined;
  const detailViolations = detail ? violations.filter((v) => v.sessionId === detail.sessionId) : [];

  return (
    <AppShell
      title={t.pageTitle}
      description={t.resultCount(filtered.length)}
      actions={
        <Button variant="secondary" size="sm" onPress={() => downloadCsv(filtered, t)} isDisabled={filtered.length === 0}>
          <Download className="size-3.5" /> {t.exportCsv}
        </Button>
      }
    >
      <div className="mb-4 max-w-xs">
        <SelectInput options={tryoutOptions} selectedKey={tryoutFilter} onSelectionChange={(k) => setTryoutFilter(k ?? "all")} />
      </div>

      {tryoutFilter !== "all" && (
        <div className="mb-6">
          <p className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-foreground">
            <Trophy className="size-4" /> {t.leaderboardTitle}
          </p>
          <div className="space-y-1.5">
            {getLeaderboardForTryout(tryoutFilter).map((row) => {
              const u = findUser(row.userId);
              return (
                <div key={row.userId} className="flex items-center gap-3 rounded-lg border border-border px-3.5 py-2.5 text-sm">
                  <span className="w-6 shrink-0 text-center font-mono text-xs font-semibold text-muted">{row.rank}</span>
                  {u && <GradientAvatar user={u} size="sm" />}
                  <span className="min-w-0 flex-1 truncate font-medium text-foreground">{u?.name ?? row.userId}</span>
                  <span className="text-xs text-muted">{t.attemptCount(row.attemptCount)}</span>
                  <span className="font-mono font-semibold text-foreground">{row.avgScore}</span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <p className="mb-2 text-sm font-semibold text-foreground">{t.historyTitle}</p>
      {filtered.length === 0 ? (
        <EmptyState icon={BarChart3} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <DataTable columns={columns} rows={filtered} rowKey={(r) => r.id} onRowClick={setDetail} />
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detailUser?.name ?? ""}
        description={detailTryout?.title}
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
              {detailUser && <GradientAvatar user={detailUser} size="lg" />}
              <div className="flex-1">
                <p className="text-sm font-semibold text-foreground">{detailUser?.name}</p>
                <p className="text-xs text-muted">{detailTryout?.title}</p>
              </div>
              {detail.irtWarning && (
                <Chip size="sm" className="bg-warning-soft text-warning-soft-foreground gap-1 font-medium">
                  <TriangleAlert className="size-3" /> {t.irtWarningBadge}
                </Chip>
              )}
            </div>
            <div className="grid grid-cols-3 gap-3 border-t border-border pt-4 text-sm">
              <div>
                <p className="text-xs text-muted">{t.detailTotalScore}</p>
                <p className="font-mono text-base font-semibold text-foreground">
                  {detail.totalScore} / {detail.maxScore}
                </p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailRank}</p>
                <p className="font-medium text-foreground">
                  {detail.rank} / {detail.participantCount}
                </p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailCompleted}</p>
                <p className="font-medium text-foreground">{formatDateTime(detail.completedAt)}</p>
              </div>
            </div>
            <div>
              <p className="mb-2 text-[13px] font-medium text-foreground">{t.subtestBreakdownTitle}</p>
              <ul className="space-y-2">
                {detail.subtestScores.map((s) => {
                  const subtest = getSubtest(s.subtestId);
                  return (
                    <li
                      key={s.subtestId}
                      className="flex items-center justify-between gap-3 rounded-lg border border-border px-3.5 py-2.5 text-sm"
                    >
                      <span className="font-medium text-foreground">{subtest?.name ?? s.subtestId}</span>
                      <span className="flex items-center gap-3 text-xs text-muted">
                        <span>{t.correctOf(s.correct, s.total)}</span>
                        <span className="font-mono font-semibold text-foreground">{s.score}</span>
                      </span>
                    </li>
                  );
                })}
              </ul>
            </div>
            <div>
              <p className="mb-2 text-[13px] font-medium text-foreground">{t.violationHistoryTitle(detailViolations.length)}</p>
              {detailViolations.length === 0 ? (
                <p className="text-sm text-muted">{t.noViolations}</p>
              ) : (
                <ul className="space-y-2">
                  {detailViolations.map((v) => (
                    <li key={v.id} className="flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger-soft px-3 py-2 text-sm">
                      <AlertTriangle className="size-3.5 shrink-0 text-danger" />
                      <span className="text-danger-soft-foreground">
                        {v.type === "screenshot_attempt" ? t.violationScreenshot : t.violationFullscreenExit}
                      </span>
                      <span className="ml-auto text-xs text-muted">{formatDateTime(v.timestamp)}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
