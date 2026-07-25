"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { ClipboardList, Pencil, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { formatDate, getProgram, getSubtestsForTryout, tryouts as initialTryouts } from "@/lib/mock-data";
import type { Tryout } from "@/lib/types";
import { useAdminTryoutsListT } from "@/lib/i18n/dict/admin-tryouts-list";

export default function TryoutsListPage() {
  const router = useRouter();
  const t = useAdminTryoutsListT();
  const [rows, setRows] = useState<Tryout[]>(initialTryouts);
  const [deleteTarget, setDeleteTarget] = useState<Tryout | null>(null);

  const columns: Column<Tryout>[] = [
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
    {
      key: "mode",
      header: t.columns.mode,
      render: (row) => <span className="text-muted">{row.scoringMode === "irt" ? t.scoringMode.irt : t.scoringMode.weighted}</span>,
    },
    { key: "subtests", header: t.columns.subtests, render: (row) => getSubtestsForTryout(row.id).length },
    { key: "participants", header: t.columns.participants, render: (row) => row.participantCount },
    { key: "activeUntil", header: t.columns.activeUntil, render: (row) => formatDate(row.activeUntil) },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (row) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <LinkButton href={`/admin/tryouts/${row.id}`} variant="ghost" size="sm" isIconOnly>
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
        <LinkButton href="/admin/tryouts/new" variant="primary" size="sm">
          <Plus className="size-3.5" /> {t.createTryout}
        </LinkButton>
      }
    >
      {rows.length === 0 ? (
        <EmptyState icon={ClipboardList} title={t.emptyTitle} />
      ) : (
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(row) => row.id}
          onRowClick={(row) => router.push(`/admin/tryouts/${row.id}`)}
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
