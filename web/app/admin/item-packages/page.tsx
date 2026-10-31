"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { CloudOff, Pencil, Plus, Shapes, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { StatusChip } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput } from "@/components/ui/form";
import { api, ApiError, errorText, type ItemPackage, type ItemPackageType } from "@/lib/api/client";
import { useItemPackagesT } from "@/lib/i18n/dict/item-packages";

const packageTypes: ItemPackageType[] = ["symbol", "image", "emoji", "shape"];

export default function ItemPackagesPage() {
  const t = useItemPackagesT();
  const router = useRouter();
  const [rows, setRows] = useState<ItemPackage[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [renaming, setRenaming] = useState<ItemPackage | null>(null);
  const [deleting, setDeleting] = useState<ItemPackage | null>(null);
  const [name, setName] = useState("");
  const [type, setType] = useState<ItemPackageType>("symbol");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.itemPackages
      .list()
      .then(({ packages }) => {
        if (cancelled) return;
        setRows(packages);
        setLoadFailed(false);
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

  const openCreate = () => {
    setName("");
    setType("symbol");
    setFieldErrors({});
    setCreateOpen(true);
  };

  const openRename = (row: ItemPackage) => {
    setName(row.name);
    setFieldErrors({});
    setRenaming(row);
  };

  const save = async () => {
    setBusy(true);
    setFieldErrors({});
    try {
      if (renaming) {
        const saved = await api.itemPackages.rename(renaming.id, name);
        setRows((prev) => prev?.map((r) => (r.id === saved.id ? saved : r)) ?? [saved]);
        toast.success(t.toastRenamed);
        setRenaming(null);
      } else {
        const created = await api.itemPackages.create({ type, name });
        toast.success(t.toastCreated);
        setCreateOpen(false);
        router.push(`/admin/item-packages/${created.id}`);
      }
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFieldErrors(err.fields);
      toast.danger(t.toastSaveFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setBusy(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleting) return;
    setBusy(true);
    try {
      await api.itemPackages.remove(deleting.id);
      setRows((prev) => prev?.filter((r) => r.id !== deleting.id) ?? []);
      toast.success(t.toastDeleted);
      setDeleting(null);
    } catch (err) {
      toast.danger(t.toastCannotDelete, { description: errorText(err, t.toastCannotDelete) });
      setDeleting(null);
    } finally {
      setBusy(false);
    }
  };

  const columns: Column<ItemPackage>[] = [
    {
      key: "name",
      header: t.columns.name,
      render: (r) => (
        <div className="flex items-center gap-3">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-control bg-accent-soft text-accent">
            <Shapes className="size-4" aria-hidden />
          </span>
          <span className="font-semibold">{r.name}</span>
        </div>
      ),
    },
    {
      key: "type",
      header: t.columns.type,
      render: (r) => <StatusChip tone="accent">{t.types[r.type]}</StatusChip>,
    },
    {
      key: "elements",
      header: t.columns.elements,
      render: (r) => (
        <span className={r.required_count > r.element_count ? "text-danger" : "text-muted"}>
          {t.elementsOfRequired(r.element_count, r.required_count)}
        </span>
      ),
    },
    { key: "status", header: t.columns.status, render: (r) => <span className="text-muted">{r.in_use ? t.inUse : t.unused}</span> },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (r) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly aria-label={t.renameAction} onPress={() => openRename(r)}>
            <Pencil className="size-4" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly aria-label={t.deleteAction} className="text-danger" isDisabled={r.in_use} onPress={() => setDeleting(r)}>
            <Trash2 className="size-4" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin", "operator"]} title={t.requireRoleTitle} deniedMessage={t.requireRoleDenied}>
      <AppShell
        title={t.title}
        description={t.description}
        actions={
          <Button variant="primary" size="sm" onPress={openCreate}>
            <Plus className="size-4" /> {t.addPackage}
          </Button>
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
          <EmptyState
            icon={Shapes}
            title={t.emptyTitle}
            description={t.emptyDescription}
            action={<Button variant="primary" size="sm" onPress={openCreate}>{t.addPackage}</Button>}
          />
        ) : (
          <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} footer={<span>{t.footerCount(rows.length)}</span>} onRowClick={(r) => router.push(`/admin/item-packages/${r.id}`)} />
        )}

        <FormModal
          isOpen={createOpen || renaming !== null}
          onOpenChange={(open) => {
            if (!open) {
              setCreateOpen(false);
              setRenaming(null);
            }
          }}
          title={renaming ? t.renameTitle : t.createTitle}
          footer={
            <>
              <Button variant="ghost" onPress={() => { setCreateOpen(false); setRenaming(null); }}>
                {t.cancel}
              </Button>
              <Button variant="primary" isDisabled={!name.trim() || busy} onPress={() => void save()}>
                {busy ? t.saving : t.save}
              </Button>
            </>
          }
        >
          {!renaming && (
            <SelectInput
              label={t.typeLabel}
              error={fieldErrors.type}
              options={packageTypes.map((p) => ({ id: p, label: t.types[p] }))}
              selectedKey={type}
              onSelectionChange={(key) => key && setType(key as ItemPackageType)}
            />
          )}
          <TextInput
            label={t.nameLabel}
            placeholder={t.namePlaceholder}
            required
            error={fieldErrors.name}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </FormModal>

        <FormModal
          isOpen={deleting !== null}
          onOpenChange={(open) => !open && setDeleting(null)}
          title={t.deleteTitle}
          description={deleting ? t.deleteBody(deleting.name) : undefined}
          size="sm"
          footer={
            <>
              <Button variant="ghost" onPress={() => setDeleting(null)}>
                {t.cancel}
              </Button>
              <Button variant="danger" isDisabled={busy} onPress={() => void confirmDelete()}>
                {busy ? t.deleting : t.deleteAction}
              </Button>
            </>
          }
        >
          {null}
        </FormModal>
      </AppShell>
    </RequireRole>
  );
}
