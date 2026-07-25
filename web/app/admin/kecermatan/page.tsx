"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { Gauge, Pencil, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { getProgram, kecermatanTests as initialTests } from "@/lib/mock-data";
import type { KecermatanTest } from "@/lib/types";
import { useAdminKecermatanListT } from "@/lib/i18n/dict/admin-kecermatan-list";

export default function KecermatanListPage() {
  const router = useRouter();
  const t = useAdminKecermatanListT();
  const [rows, setRows] = useState<KecermatanTest[]>(initialTests);
  const [deleteTarget, setDeleteTarget] = useState<KecermatanTest | null>(null);

  const columns: Column<KecermatanTest>[] = [
    {
      key: "title",
      header: t.columns.title,
      render: (row) => (
        <div className="flex items-center gap-3">
          <div className="size-8 shrink-0 rounded-md" style={{ background: `linear-gradient(135deg, ${row.coverFrom}, ${row.coverTo})` }} />
          <span className="font-medium">{row.title}</span>
        </div>
      ),
    },
    { key: "program", header: t.columns.program, render: (row) => { const p = getProgram(row.programId); return p ? <ProgramBadge code={p.code} /> : null; } },
    { key: "structure", header: t.columns.structureHeader, render: (row) => <span className="text-muted">{t.columns.structure(row.kolomCount, row.soalPerKolom)}</span> },
    { key: "duration", header: t.columns.durationHeader, render: (row) => t.columns.duration(Math.round(row.kolomDurationSeconds / 60)) },
    { key: "participants", header: t.columns.participants, render: (row) => row.participantCount },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (row) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <LinkButton href={`/admin/kecermatan/${row.id}`} variant="ghost" size="sm" isIconOnly>
            <Pencil className="size-3.5" />
          </LinkButton>
          <Button
            variant="ghost"
            size="sm"
            isIconOnly
            className="text-danger"
            isDisabled={row.participantCount > 0}
            onPress={() => setDeleteTarget(row)}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <AppShell
      title={t.title}
      description={t.description(rows.length)}
      actions={
        <LinkButton href="/admin/kecermatan/new" variant="primary" size="sm">
          <Plus className="size-3.5" /> {t.createTest}
        </LinkButton>
      }
    >
      {rows.length === 0 ? (
        <EmptyState icon={Gauge} title={t.emptyTitle} />
      ) : (
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(row) => row.id}
          onRowClick={(row) => router.push(`/admin/kecermatan/${row.id}`)}
        />
      )}

      <FormModal
        isOpen={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t.deleteTitle}
        description={deleteTarget?.title}
        footer={
          <>
            <Button variant="ghost" onPress={() => setDeleteTarget(null)}>
              {t.cancel}
            </Button>
            <Button
              variant="danger"
              onPress={() => {
                setRows((prev) => prev.filter((row) => row.id !== deleteTarget?.id));
                toast.success(t.toastDeleted);
                setDeleteTarget(null);
              }}
            >
              {t.confirmDelete}
            </Button>
          </>
        }
      >
        <p className="text-sm text-muted">{t.deleteWarning}</p>
      </FormModal>
    </AppShell>
  );
}
