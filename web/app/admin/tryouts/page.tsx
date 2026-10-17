"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { ClipboardList, CloudOff, Lock, Pencil, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge, TryoutStatusBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { api, errorText, type Program, type TryoutSummary } from "@/lib/api/client";
import { useAdminTryoutsListT } from "@/lib/i18n/dict/admin-tryouts-list";

const PAGE_SIZE = 50;

export default function TryoutsListPage() {
  const router = useRouter();
  const t = useAdminTryoutsListT();

  const [programs, setPrograms] = useState<Program[] | null>(null);
  const [rows, setRows] = useState<TryoutSummary[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState<TryoutSummary | null>(null);
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
    api.tryouts
      .list({ limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.tryouts);
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
      const page = await api.tryouts.list({ limit: PAGE_SIZE, offset: rows?.length ?? 0 });
      setRows((prev) => [...(prev ?? []), ...page.tryouts]);
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const programOf = (id: string) => programs?.find((program) => program.id === id);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.tryouts.remove(deleteTarget.id);
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

  const columns: Column<TryoutSummary>[] = [
    { key: "title", header: t.columns.title, render: (row) => <span className="font-medium">{row.title}</span> },
    {
      key: "program",
      header: t.columns.program,
      render: (row) => {
        const program = programOf(row.program_id);
        return program ? <ProgramBadge code={program.code} /> : null;
      },
    },
    {
      key: "mode",
      header: t.columns.mode,
      render: (row) => <span className="text-muted">{row.scoring_mode === "irt" ? t.scoringMode.irt : t.scoringMode.weighted}</span>,
    },
    { key: "subtests", header: t.columns.subtests, render: (row) => row.subtest_count },
    {
      key: "status",
      header: t.columns.status,
      render: (row) => (
        <div className="flex items-center gap-2">
          <TryoutStatusBadge status={row.status} />
          {row.locked && (
            <span className="flex items-center gap-1 text-xs text-muted" title={t.lockedHint}>
              <Lock className="size-3.5" /> <span className="sr-only">{t.lockedLabel}</span>
            </span>
          )}
        </div>
      ),
    },
    {
      key: "activeUntil",
      header: t.columns.activeUntil,
      render: (row) => new Date(row.active_until).toLocaleDateString(),
    },
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
            isDisabled={row.locked}
            onPress={() => setDeleteTarget(row)}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin", "operator"]} title={t.requireRoleTitle} deniedMessage={t.requireRoleDenied}>
      <AppShell
        title={t.title}
        description={t.description(total)}
        actions={
          <LinkButton href="/admin/tryouts/new" variant="primary" size="sm">
            <Plus className="size-3.5" /> {t.createTryout}
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
          <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
        ) : rows.length === 0 ? (
          <EmptyState icon={ClipboardList} title={t.emptyTitle} description={t.emptyDescription} />
        ) : (
          <>
            <DataTable columns={columns} rows={rows} rowKey={(row) => row.id} onRowClick={(row) => router.push(`/admin/tryouts/${row.id}`)} />
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
          <p className="text-sm text-muted">{t.deleteWarning}</p>
        </FormModal>
      </AppShell>
    </RequireRole>
  );
}
