"use client";

import { useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { BarChart3, CloudOff, Download } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput } from "@/components/ui/form";
import {
  api,
  type KecermatanStaffResult,
  type KecermatanStaffResultDetail,
  type KecermatanTestSummary,
  type Program,
} from "@/lib/api/client";
import { useKecermatanResultsT } from "@/lib/i18n/dict/results";

const PAGE_SIZE = 100;

const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

// Exports the rows on screen only; the bulk CSV/XLSX export is P21.
function downloadCsv(rows: KecermatanStaffResult[], t: ReturnType<typeof useKecermatanResultsT>) {
  const lines = rows.map((r) =>
    [
      r.test_title,
      r.student_name,
      r.attempt_number,
      r.benar_total,
      r.salah_total,
      r.kosong_total,
      r.scores.kecepatan,
      r.scores.ketahanan,
      r.scores.ketelitian,
      r.scores.nilai_akhir,
      r.completed_at,
    ]
      .map((v) => `"${String(v).replace(/"/g, '""')}"`)
      .join(","),
  );
  const csv = [t.csvHeaders.join(","), ...lines].join("\n");
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "hasil-kecermatan.csv";
  a.click();
  URL.revokeObjectURL(url);
}

export default function KecermatanResultsPage() {
  const t = useKecermatanResultsT();
  const [tests, setTests] = useState<KecermatanTestSummary[]>([]);
  const [programs, setPrograms] = useState<Program[]>([]);
  const [testFilter, setTestFilter] = useState("all");
  const [programFilter, setProgramFilter] = useState("all");
  const [rows, setRows] = useState<KecermatanStaffResult[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [detail, setDetail] = useState<KecermatanStaffResult | null>(null);
  const [breakdown, setBreakdown] = useState<KecermatanStaffResultDetail | null>(null);

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
    let cancelled = false;
    api.kecermatanResults
      .list({
        test_id: testFilter === "all" ? undefined : testFilter,
        program_id: programFilter === "all" ? undefined : programFilter,
        limit: PAGE_SIZE,
      })
      .then((page) => {
        if (cancelled) return;
        setRows(page.results);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [testFilter, programFilter, reloadKey]);

  useEffect(() => {
    if (!detail) return;
    let cancelled = false;
    api.kecermatanResults
      .get(detail.session_id)
      .then((loaded) => {
        if (!cancelled) setBreakdown(loaded);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [detail]);

  const testOptions = [{ id: "all", label: t.allTests }, ...tests.map((test) => ({ id: test.id, label: test.title }))];
  const programOptions = [{ id: "all", label: t.allPrograms }, ...programs.map((p) => ({ id: p.id, label: p.name }))];

  const columns: Column<KecermatanStaffResult>[] = [
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
    { key: "test", header: t.colTest, render: (r) => r.test_title },
    { key: "attempt", header: t.colAttempt, render: (r) => <span className="text-muted">#{r.attempt_number}</span> },
    { key: "bsk", header: t.colBsk, render: (r) => <span className="font-mono text-label">{r.benar_total}/{r.salah_total}/{r.kosong_total}</span> },
    { key: "nilai", header: t.colFinalScore, render: (r) => <span className="font-mono font-semibold text-foreground">{r.scores.nilai_akhir}</span> },
    { key: "completedAt", header: t.colCompleted, render: (r) => formatDateTime(r.completed_at) },
  ];

  const body =
    loadFailed ? (
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
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(r) => r.session_id}
        onRowClick={(row) => {
          setBreakdown(null);
          setDetail(row);
        }}
      />
    );

  return (
    <AppShell
      title={t.pageTitle}
      description={t.resultCount(total)}
      actions={
        <Button variant="secondary" size="sm" onPress={() => rows && downloadCsv(rows, t)} isDisabled={!rows || rows.length === 0}>
          <Icon icon={Download} size="xs" /> {t.exportCsv}
        </Button>
      }
    >
      <div className="mb-4 flex flex-wrap gap-3">
        <div className="w-full max-w-xs">
          <SelectInput
            label={t.fieldTest}
            options={testOptions}
            selectedKey={testFilter}
            onSelectionChange={(k) => {
              setRows(null);
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
              setProgramFilter(k ?? "all");
            }}
          />
        </div>
      </div>

      {body}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detail?.student_name ?? ""}
        description={detail?.test_title}
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
                <p className="text-label text-muted">
                  {detail.test_title} — {t.attemptLabel(detail.attempt_number)}
                </p>
              </div>
            </div>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-4 rounded-tile border border-border p-5 text-small sm:grid-cols-4">
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailSpeed}</dt>
                <dd className="font-mono text-title font-semibold text-foreground">{detail.scores.kecepatan}</dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailStamina}</dt>
                <dd className="font-mono text-title font-semibold text-foreground">{detail.scores.ketahanan}</dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailAccuracy}</dt>
                <dd className="font-mono text-title font-semibold text-foreground">{detail.scores.ketelitian}</dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.detailFinalScore}</dt>
                <dd className="font-mono text-title font-semibold text-accent">{detail.scores.nilai_akhir}</dd>
              </div>
            </dl>
            <div>
              <h3 className="mb-3 font-display text-title">{t.columnBreakdownTitle}</h3>
              {breakdown === null ? (
                <p className="text-caption text-muted">{t.loading}</p>
              ) : (
                <ul className="space-y-2">
                  {breakdown.kolom.map((k) => (
                    <li key={k.number} className="flex items-center justify-between gap-3 rounded-control border border-border px-4 py-3 text-small">
                      <span className="font-medium text-foreground">{t.columnLabel(k.number)}</span>
                      <span className="flex items-center gap-3 font-mono text-label text-muted">
                        <span className="text-success">{t.correct(k.benar)}</span>
                        <span className="text-danger">{t.incorrect(k.salah)}</span>
                        <span>{t.blank(k.kosong)}</span>
                      </span>
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
