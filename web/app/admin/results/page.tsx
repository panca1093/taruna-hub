"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { BarChart3, CloudOff, Download, Trophy, TriangleAlert } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { useBulkSheetPdf } from "@/components/capture-stage";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { StatusChip } from "@/components/status-badge";
import { TryoutResultCaptureSheet } from "@/components/tryout-result-capture-sheet";
import { FilterMenu, SearchField, type FilterField, type FilterValue } from "@/components/filter-menu";
import { api, errorText, type Leaderboard, type Program, type StaffResultStudentRow, type TryoutSummary } from "@/lib/api/client";
import { useResultsT } from "@/lib/i18n/dict/results";

const PAGE_SIZE = 50;
const BOARD_SIZE = 20;
const MAX_SELECTED = 30;
const SEARCH_DEBOUNCE_MS = 300;

const rowKey = (r: StaffResultStudentRow) => `${r.student_id}:${r.tryout_id}`;

const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

export default function ResultsPage() {
  const t = useResultsT();
  const [tryouts, setTryouts] = useState<TryoutSummary[]>([]);
  const [tryoutFilter, setTryoutFilter] = useState("all");
  const [programs, setPrograms] = useState<Program[]>([]);
  const [programFilter, setProgramFilter] = useState("all");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const bulk = useBulkSheetPdf();
  const router = useRouter();
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [rows, setRows] = useState<StaffResultStudentRow[] | null>(null);
  const [total, setTotal] = useState(0);
  const [board, setBoard] = useState<Leaderboard | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const generationRef = useRef(0);
  const loadingMoreRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.tryouts.list({ limit: 100 }), api.programs.list()])
      .then(([page, programList]) => {
        if (cancelled) return;
        setTryouts(page.tryouts);
        setPrograms(programList.programs);
      })
      .catch(() => {
        // The filter is a convenience; the list below still loads and reports its own failure.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      const next = searchInput.trim();
      if (next === search) return;
      setRows(null);
      setSelected(new Set());
      setLoadFailed(false);
      setSearch(next);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [searchInput, search]);

  useEffect(() => {
    let cancelled = false;
    generationRef.current += 1;
    const tryoutId = tryoutFilter === "all" ? undefined : tryoutFilter;
    api.results
      .list({ tryout_id: tryoutId, program_id: programFilter === "all" ? undefined : programFilter, q: search || undefined, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        loadingMoreRef.current = false;
        setLoadingMore(false);
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
  }, [tryoutFilter, programFilter, search, reloadKey]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const changeTryout = (next: string) => {
    setRows(null);
    setSelected(new Set());
    setBoard(null);
    setLoadFailed(false);
    setTryoutFilter(next);
  };

  const changeProgram = (next: string) => {
    setRows(null);
    setSelected(new Set());
    setLoadFailed(false);
    setProgramFilter(next);
  };

  const working = bulk.status === "working";

  const downloadPdf = async () => {
    const chosen = (rows ?? []).filter((r) => selected.has(rowKey(r)));
    if (chosen.length === 0) return;
    const saved = await bulk.run(
      chosen,
      async (row) => {
        const detail = await api.results.student(row.student_id, row.tryout_id);
        if (detail.attempts.length === 0) throw new Error("no completed attempt");
        return <TryoutResultCaptureSheet detail={detail} attempt={detail.attempts[0]} />;
      },
      `hasil-tryout-${chosen.length}-siswa-${new Date().toLocaleDateString("sv-SE", { timeZone: "Asia/Jakarta" })}.pdf`,
    );
    if (saved) toast.success(t.bulkDone(chosen.length));
  };

  const loadMore = async () => {
    if (loadingMoreRef.current) return;
    const generation = generationRef.current;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const page = await api.results.list({
        tryout_id: tryoutFilter === "all" ? undefined : tryoutFilter,
        program_id: programFilter === "all" ? undefined : programFilter,
        q: search || undefined,
        limit: PAGE_SIZE,
        offset: rows?.length ?? 0,
      });
      if (generationRef.current !== generation) return;
      setRows((prev) => {
        const seen = new Set((prev ?? []).map(rowKey));
        return [...(prev ?? []), ...page.results.filter((row) => !seen.has(rowKey(row)))];
      });
      setTotal(page.total);
    } catch (err) {
      if (generationRef.current !== generation) return;
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      if (generationRef.current === generation) {
        loadingMoreRef.current = false;
        setLoadingMore(false);
      }
    }
  };

  const filterFields: FilterField[] = [
    { key: "tryout", label: t.fieldTryout, allLabel: t.allTryouts, options: tryouts.map((tr) => ({ id: tr.id, label: tr.title })) },
    { key: "program", label: t.fieldProgram, allLabel: t.allPrograms, options: programs.map((p) => ({ id: p.id, label: p.name })) },
  ];
  const filterValue = { tryout: tryoutFilter === "all" ? "" : tryoutFilter, program: programFilter === "all" ? "" : programFilter };
  const filtering = search !== "" || tryoutFilter !== "all" || programFilter !== "all";
  const applyFilter = (next: FilterValue) => {
    if ((next.tryout ?? "") !== filterValue.tryout) changeTryout(next.tryout || "all");
    if ((next.program ?? "") !== filterValue.program) changeProgram(next.program || "all");
  };

  const columns: Column<StaffResultStudentRow>[] = [
    {
      key: "student",
      header: t.colStudent,
      render: (r) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={{ name: r.student_name, avatar: r.student_avatar }} size="sm" />
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
    { key: "attempts", header: t.colAttempt, render: (r) => <span className="font-mono text-label">{r.attempts_completed}</span> },
    { key: "average", header: t.colAverage, render: (r) => <span className="font-mono font-semibold text-foreground">{r.average_score}</span> },
    { key: "latest", header: t.colLatest, render: (r) => <span className="font-mono text-label">{r.latest_score}</span> },
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
    { key: "lastCompleted", header: t.colLastCompleted, render: (r) => formatDateTime(r.last_completed_at) },
  ];

  return (
    <AppShell
      title={t.pageTitle}
      description={rows === null ? undefined : t.resultCount(total)}
      actions={
        <Button variant="primary" size="sm" isDisabled={selected.size === 0 || working} onPress={() => void downloadPdf()}>
          <Icon icon={Download} size="xs" /> {working ? t.bulkProgress(bulk.page, bulk.total) : t.bulkPdf(selected.size)}
        </Button>
      }
    >
      <div className="mb-4 flex items-center gap-2">
        <SearchField value={searchInput} onChange={(value) => setSearchInput(value.slice(0, 100))} placeholder={t.searchPlaceholder} />
        <FilterMenu fields={filterFields} value={filterValue} onChange={applyFilter} align="start" />
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
                <GradientAvatar user={{ name: row.student_name, avatar: row.student_avatar }} size="sm" />
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
        <EmptyState icon={BarChart3} title={filtering ? t.noMatchTitle : t.emptyTitle} description={filtering ? t.noMatchDescription : t.emptyDescription} />
      ) : (
        <>
          {bulk.status === "error" && (
            <div role="alert" className="mb-3 flex flex-wrap items-center justify-between gap-3 rounded-tile border border-danger bg-danger-soft px-4 py-3 text-caption text-danger">
              <span>
                <strong className="font-medium">{t.bulkFailedTitle}.</strong> {t.bulkFailed(bulk.done, bulk.total)}
              </span>
              <Button variant="secondary" size="sm" onPress={() => void downloadPdf()}>
                {t.bulkRetry}
              </Button>
            </div>
          )}
          {selected.size >= MAX_SELECTED && <p className="mb-3 text-label text-muted">{t.bulkCap(MAX_SELECTED)}</p>}
          <DataTable
            columns={columns}
            rows={rows}
            rowKey={rowKey}
            onRowClick={(r) => router.push(`/admin/results/${r.tryout_id}/${r.student_id}`)}
            selectedKeys={selected}
            onSelectionChange={setSelected}
            maxSelected={MAX_SELECTED}
            isSelectionDisabled={working}
            selectAllLabel={t.selectAll}
            selectRowLabel={(r) => t.selectRow(r.student_name)}
          />
          {rows.length < total && (
            <div className="mt-4 flex justify-center">
              <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                {loadingMore ? t.loadingMore : t.loadMore}
              </Button>
            </div>
          )}
        </>
      )}
      {bulk.stage}
    </AppShell>
  );
}
