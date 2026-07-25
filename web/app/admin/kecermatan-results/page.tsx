"use client";

import { useMemo, useState } from "react";
import { Button } from "@heroui/react";
import { BarChart3, Download } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput } from "@/components/ui/form";
import {
  findUser,
  formatDateTime,
  getKecermatanTest,
  kecermatanResults,
  kecermatanTests,
  programs,
} from "@/lib/mock-data";
import { useKecermatanResultsT } from "@/lib/i18n/dict/results";
import type { KecermatanResult } from "@/lib/types";

function downloadCsv(rows: KecermatanResult[], t: ReturnType<typeof useKecermatanResultsT>) {
  const lines = rows.map((r) =>
    [
      getKecermatanTest(r.kecermatanTestId)?.title ?? r.kecermatanTestId,
      findUser(r.userId)?.name ?? r.userId,
      r.attemptNumber,
      r.benarTotal,
      r.salahTotal,
      r.kosongTotal,
      r.kecepatanScore,
      r.ketahananScore,
      r.ketelitianScore,
      r.nilaiAkhirScore,
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
  a.download = "hasil-kecermatan.csv";
  a.click();
  URL.revokeObjectURL(url);
}

export default function KecermatanResultsPage() {
  const t = useKecermatanResultsT();
  const testOptions = [{ id: "all", label: t.allTests }, ...kecermatanTests.map((tst) => ({ id: tst.id, label: tst.title }))];
  const programOptions = [{ id: "all", label: t.allPrograms }, ...programs.map((p) => ({ id: p.id, label: p.name }))];
  const [testFilter, setTestFilter] = useState("all");
  const [programFilter, setProgramFilter] = useState("all");
  const [detail, setDetail] = useState<KecermatanResult | null>(null);

  const filtered = useMemo(() => {
    return kecermatanResults.filter((r) => {
      const test = getKecermatanTest(r.kecermatanTestId);
      if (testFilter !== "all" && r.kecermatanTestId !== testFilter) return false;
      if (programFilter !== "all" && test?.programId !== programFilter) return false;
      return true;
    });
  }, [testFilter, programFilter]);

  const columns: Column<KecermatanResult>[] = [
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
    { key: "test", header: t.colTest, render: (r) => getKecermatanTest(r.kecermatanTestId)?.title ?? r.kecermatanTestId },
    { key: "attempt", header: t.colAttempt, render: (r) => <span className="text-muted">#{r.attemptNumber}</span> },
    { key: "bsk", header: t.colBsk, render: (r) => <span className="font-mono text-xs">{r.benarTotal}/{r.salahTotal}/{r.kosongTotal}</span> },
    { key: "nilai", header: t.colFinalScore, render: (r) => <span className="font-mono font-semibold text-foreground">{r.nilaiAkhirScore}</span> },
    { key: "completedAt", header: t.colCompleted, render: (r) => formatDateTime(r.completedAt) },
  ];

  const detailTest = detail ? getKecermatanTest(detail.kecermatanTestId) : undefined;
  const detailUser = detail ? findUser(detail.userId) : undefined;

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
      <div className="mb-4 flex flex-wrap gap-3">
        <div className="w-full max-w-xs">
          <SelectInput label={t.fieldTest} options={testOptions} selectedKey={testFilter} onSelectionChange={(k) => setTestFilter(k ?? "all")} />
        </div>
        <div className="w-full max-w-xs">
          <SelectInput label={t.fieldProgram} options={programOptions} selectedKey={programFilter} onSelectionChange={(k) => setProgramFilter(k ?? "all")} />
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={BarChart3} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <DataTable columns={columns} rows={filtered} rowKey={(r) => r.id} onRowClick={setDetail} />
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detailUser?.name ?? ""}
        description={detailTest?.title}
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
                <p className="text-xs text-muted">{detailTest?.title} — {t.attemptLabel(detail.attemptNumber)}</p>
              </div>
            </div>
            <div className="grid grid-cols-4 gap-3 border-t border-border pt-4 text-sm">
              <div>
                <p className="text-xs text-muted">{t.detailSpeed}</p>
                <p className="font-mono text-base font-semibold text-foreground">{detail.kecepatanScore}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailStamina}</p>
                <p className="font-mono text-base font-semibold text-foreground">{detail.ketahananScore}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailAccuracy}</p>
                <p className="font-mono text-base font-semibold text-foreground">{detail.ketelitianScore}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailFinalScore}</p>
                <p className="font-mono text-base font-semibold text-accent">{detail.nilaiAkhirScore}</p>
              </div>
            </div>
            <div>
              <p className="mb-2 text-[13px] font-medium text-foreground">{t.columnBreakdownTitle}</p>
              <ul className="space-y-2">
                {detail.kolomResults.map((k) => (
                  <li key={k.kolomNumber} className="flex items-center justify-between gap-3 rounded-lg border border-border px-3.5 py-2.5 text-sm">
                    <span className="font-medium text-foreground">{t.columnLabel(k.kolomNumber)}</span>
                    <span className="flex items-center gap-3 font-mono text-xs text-muted">
                      <span className="text-success">{t.correct(k.benar)}</span>
                      <span className="text-danger">{t.incorrect(k.salah)}</span>
                      <span>{t.blank(k.kosong)}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
