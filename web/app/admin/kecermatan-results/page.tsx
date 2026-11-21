"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { BarChart3, CloudOff, Download } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { useBulkSheetPdf } from "@/components/capture-stage";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { KecermatanResultCaptureSheet } from "@/components/kecermatan-result-capture-sheet";
import { SelectInput, TextInput } from "@/components/ui/form";
import { api, errorText, type KecermatanStaffStudentRow, type KecermatanTestSummary, type Program } from "@/lib/api/client";
import { useKecermatanResultsT } from "@/lib/i18n/dict/results";

const PAGE_SIZE = 100;
const MAX_SELECTED = 30;
const SEARCH_DEBOUNCE_MS = 300;

const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

const rowKey = (r: KecermatanStaffStudentRow) => `${r.student_id}:${r.test_id}`;

export default function KecermatanResultsPage() {
  const t = useKecermatanResultsT();
  const router = useRouter();
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const bulk = useBulkSheetPdf();
  const [tests, setTests] = useState<KecermatanTestSummary[]>([]);
  const [programs, setPrograms] = useState<Program[]>([]);
  const [testFilter, setTestFilter] = useState("all");
  const [programFilter, setProgramFilter] = useState("all");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [rows, setRows] = useState<KecermatanStaffStudentRow[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const generationRef = useRef(0);
  const loadingMoreRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.kecermatan.list({ limit: 100 }), api.programs.list()])
      .then(([testPage, programList]) => {
        if (cancelled) return;
        setTests(testPage.tests);
        setPrograms(programList.programs);
      })
      .catch(() => {
        // The filters are a convenience; the list below still loads and reports its own failure.
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

  const filters = {
    test_id: testFilter === "all" ? undefined : testFilter,
    program_id: programFilter === "all" ? undefined : programFilter,
    q: search || undefined,
  };

  useEffect(() => {
    let cancelled = false;
    generationRef.current += 1;
    api.kecermatanResults
      .list({
        test_id: testFilter === "all" ? undefined : testFilter,
        program_id: programFilter === "all" ? undefined : programFilter,
        q: search || undefined,
        limit: PAGE_SIZE,
      })
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
    return () => {
      cancelled = true;
    };
  }, [testFilter, programFilter, search, reloadKey]);

  const loadMore = async () => {
    if (loadingMoreRef.current) return;
    const generation = generationRef.current;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const page = await api.kecermatanResults.list({ ...filters, limit: PAGE_SIZE, offset: rows?.length ?? 0 });
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

  const working = bulk.status === "working";

  const downloadPdf = async () => {
    const chosen = (rows ?? []).filter((r) => selected.has(rowKey(r)));
    if (chosen.length === 0) return;
    const saved = await bulk.run(
      chosen,
      async (row) => <KecermatanResultCaptureSheet detail={await api.kecermatanResults.student(row.student_id, row.test_id)} />,
      `hasil-kecermatan-${chosen.length}-siswa-${new Date().toLocaleDateString("sv-SE", { timeZone: "Asia/Jakarta" })}.pdf`,
    );
    if (saved) toast.success(t.bulkDone(chosen.length));
  };

  const testOptions = [{ id: "all", label: t.allTests }, ...tests.map((test) => ({ id: test.id, label: test.title }))];
  const programOptions = [{ id: "all", label: t.allPrograms }, ...programs.map((p) => ({ id: p.id, label: p.name }))];

  const columns: Column<KecermatanStaffStudentRow>[] = [
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
    { key: "test", header: t.colTest, render: (r) => r.test_title },
    { key: "attempts", header: t.colAttempts, render: (r) => <span className="font-mono text-label">{r.attempts_completed}</span> },
    { key: "combined", header: t.colCombined, render: (r) => <span className="font-mono font-semibold text-foreground">{r.progress.nilai_gabungan}</span> },
    { key: "speed", header: t.colSpeed, render: (r) => <span className="font-mono text-label">{r.progress.kecepatan}</span> },
    { key: "stamina", header: t.colStamina, render: (r) => <span className="font-mono text-label">{r.progress.ketahanan}</span> },
    { key: "accuracy", header: t.colAccuracy, render: (r) => <span className="font-mono text-label">{r.progress.ketelitian}</span> },
    { key: "quota", header: t.colQuota, render: (r) => <span className="font-mono text-label">{r.remaining_attempts}</span> },
    { key: "lastCompleted", header: t.colLastCompleted, render: (r) => formatDateTime(r.last_completed_at) },
  ];

  const body = loadFailed ? (
    <EmptyState
      icon={CloudOff}
      title={t.loadErrorTitle}
      description={t.loadErrorDescription}
      action={
        <Button
          variant="primary"
          size="sm"
          onPress={() => {
            setLoadFailed(false);
            setRows(null);
            setReloadKey((key) => key + 1);
          }}
        >
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
      {bulk.status === "error" && (
        <div role="alert" className="mb-3 flex flex-wrap items-center justify-between gap-3 rounded-tile border border-danger bg-danger-soft px-4 py-3 text-caption text-danger">
          <span>
            <strong className="font-medium">{t.bulkFailedTitle}.</strong> {t.bulkFailed(Math.max(bulk.page - 1, 0), bulk.total)}
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
        onRowClick={(row) => router.push(`/admin/kecermatan-results/${row.test_id}/${row.student_id}`)}
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
  );

  return (
    <AppShell
      title={t.pageTitle}
      description={t.resultCount(total)}
      actions={
        <Button variant="primary" size="sm" isDisabled={selected.size === 0 || working} onPress={() => void downloadPdf()}>
          <Icon icon={Download} size="xs" /> {working ? t.bulkProgress(bulk.page, bulk.total) : t.bulkPdf(selected.size)}
        </Button>
      }
    >
      <div className="mb-4 flex flex-wrap gap-3">
        <div className="w-full max-w-xs">
          <TextInput
            label={t.fieldSearch}
            placeholder={t.searchPlaceholder}
            type="search"
            maxLength={100}
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
          />
        </div>
        <div className="w-full max-w-xs">
          <SelectInput
            label={t.fieldTest}
            options={testOptions}
            selectedKey={testFilter}
            onSelectionChange={(k) => {
              setRows(null);
              setSelected(new Set());
              setLoadFailed(false);
              setTestFilter(k ?? "all");
            }}
          />
        </div>
        <div className="w-full max-w-xs">
          <SelectInput
            label={t.fieldProgram}
            options={programOptions}
            selectedKey={programFilter}
            onSelectionChange={(k) => {
              setRows(null);
              setSelected(new Set());
              setLoadFailed(false);
              setProgramFilter(k ?? "all");
            }}
          />
        </div>
      </div>

      {body}
      {bulk.stage}
    </AppShell>
  );
}
