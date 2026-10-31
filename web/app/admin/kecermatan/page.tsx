"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { CloudOff, Gauge, Lock, Pencil, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { api, errorText, type KecermatanTestSummary, type Program } from "@/lib/api/client";
import { useAdminKecermatanListT } from "@/lib/i18n/dict/admin-kecermatan-list";

const PAGE_SIZE = 50;

export default function KecermatanListPage() {
  const router = useRouter();
  const t = useAdminKecermatanListT();
  const [programs, setPrograms] = useState<Program[] | null>(null);
  const [rows, setRows] = useState<KecermatanTestSummary[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState<KecermatanTestSummary | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.programs
      .list()
      .then(({ programs }) => {
        if (!cancelled) setPrograms(programs);
      })
      .catch(() => {
        if (!cancelled) setPrograms([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    api.kecermatan
      .list({ limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.tests);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const page = await api.kecermatan.list({ limit: PAGE_SIZE, offset: rows?.length ?? 0 });
      setRows((prev) => [...(prev ?? []), ...page.tests]);
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.kecermatan.remove(deleteTarget.id);
      setRows((prev) => prev?.filter((row) => row.id !== deleteTarget.id) ?? []);
      setTotal((count) => Math.max(0, count - 1));
      toast.success(t.toastDeleted);
      setDeleteTarget(null);
    } catch (err) {
      toast.danger(t.toastDeleteFailedTitle, { description: errorText(err, t.toastDeleteFailedDescription) });
    } finally {
      setDeleting(false);
    }
  };

  const columns: Column<KecermatanTestSummary>[] = [
    {
      key: "title",
      header: t.columns.title,
      render: (row) => (
        <div className="flex items-center gap-3">
          <div className="size-8 shrink-0 rounded-control" style={{ background: `linear-gradient(135deg, ${row.cover_from}, ${row.cover_to})` }} />
          <span className="font-semibold">{row.title}</span>
          {row.locked && (
            <span className="flex items-center gap-1 text-caption text-muted" title={t.lockedHint}>
              <Lock className="size-3.5" /> <span className="sr-only">{t.lockedLabel}</span>
            </span>
          )}
        </div>
      ),
    },
    {
      key: "program",
      header: t.columns.program,
      render: (row) => {
        const program = programs?.find((p) => p.id === row.program_id);
        return program ? <ProgramBadge code={program.code} /> : null;
      },
    },
    { key: "structure", header: t.columns.structureHeader, render: (row) => <span className="text-muted">{t.columns.structure(row.kolom_count, row.soal_per_kolom)}</span> },
    { key: "duration", header: t.columns.durationHeader, render: (row) => t.columns.duration(Math.round(row.kolom_duration_seconds / 60)) },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (row) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <LinkButton href={`/admin/kecermatan/${row.id}`} variant="ghost" size="sm" isIconOnly>
            <Pencil className="size-4" />
          </LinkButton>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" isDisabled={row.locked} onPress={() => setDeleteTarget(row)}>
            <Trash2 className="size-4" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <AppShell
      title={t.title}
      description={t.description(total)}
      actions={
        <LinkButton href="/admin/kecermatan/new" variant="primary" size="sm">
          <Plus className="size-4" /> {t.createTest}
        </LinkButton>
      }
    >
      {loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={<Button variant="primary" size="sm" onPress={retry}>{t.retry}</Button>}
        />
      ) : rows === null ? (
        <p className="py-14 text-center text-small text-muted">{t.loading}</p>
      ) : rows.length === 0 ? (
        <EmptyState icon={Gauge} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <>
          <DataTable columns={columns} rows={rows} rowKey={(row) => row.id} footer={<span>{t.footerCount(rows.length, total)}</span>} onRowClick={(row) => router.push(`/admin/kecermatan/${row.id}`)} />
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
        isOpen={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t.deleteTitle}
        description={deleteTarget?.title}
        footer={
          <>
            <Button variant="ghost" onPress={() => setDeleteTarget(null)}>
              {t.cancel}
            </Button>
            <Button variant="danger" onPress={confirmDelete} isDisabled={deleting}>
              {t.confirmDelete}
            </Button>
          </>
        }
      >
        <p className="text-small text-muted">{t.deleteWarning}</p>
      </FormModal>
    </AppShell>
  );
}
