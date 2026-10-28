"use client";

import { useEffect, useState } from "react";
import { Button, Chip, Switch, toast } from "@heroui/react";
import { CloudOff, Package, Pencil, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { ProgramBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, SwitchInput, TextInput, TextareaInput } from "@/components/ui/form";
import { api, ApiError, errorText, type Product, type Program, type TryoutSummary } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useProductsT } from "@/lib/i18n/dict/products";
import { cn } from "@/lib/cn";

const PAGE_SIZE = 50;
const dateInput = (iso: string | null) => (iso ? iso.slice(0, 10) : "");

interface ProductForm {
  type: "individual" | "bundle";
  title: string;
  description: string;
  programId: string;
  tryoutIds: string[];
  price: string;
  originalPrice: string;
  activeFrom: string;
  activeUntil: string;
  isActive: boolean;
}

const blankForm: ProductForm = {
  type: "individual",
  title: "",
  description: "",
  programId: "",
  tryoutIds: [],
  price: "",
  originalPrice: "",
  activeFrom: "",
  activeUntil: "",
  isActive: false,
};

export default function ProductsPage() {
  const t = useProductsT();

  const [programs, setPrograms] = useState<Program[] | null>(null);
  const [rows, setRows] = useState<Product[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Product | null>(null);
  const [form, setForm] = useState<ProductForm>(blankForm);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const [choiceTryouts, setChoiceTryouts] = useState<TryoutSummary[] | null>(null);
  const [choiceFailed, setChoiceFailed] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<Product | null>(null);
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
    api.products
      .list({ limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.products);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  // Only published tryouts of the chosen program can be attached, so the picker asks the API.
  useEffect(() => {
    if (!modalOpen || form.programId === "") return;
    let cancelled = false;
    api.tryouts
      .list({ status: "published", program_id: form.programId, limit: 100 })
      .then((page) => {
        if (!cancelled) setChoiceTryouts(page.tryouts);
      })
      .catch(() => {
        if (!cancelled) setChoiceFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [modalOpen, form.programId]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const page = await api.products.list({ limit: PAGE_SIZE, offset: rows?.length ?? 0 });
      setRows((prev) => [...(prev ?? []), ...page.products]);
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const programOf = (id: string) => programs?.find((program) => program.id === id);

  const openCreate = () => {
    const preferred = programs?.[0]?.id ?? "";
    setEditing(null);
    setFormErrors({});
    setForm({ ...blankForm, programId: preferred });
    setChoiceTryouts(null);
    setChoiceFailed(false);
    setModalOpen(true);
  };

  const openEdit = (product: Product) => {
    setEditing(product);
    setFormErrors({});
    setForm({
      type: product.type,
      title: product.title,
      description: product.description,
      programId: product.program_id,
      tryoutIds: product.tryouts.map((tryout) => tryout.id),
      price: product.price,
      originalPrice: product.original_price ?? "",
      activeFrom: dateInput(product.active_from),
      activeUntil: dateInput(product.active_until),
      isActive: product.is_active,
    });
    setChoiceTryouts(null);
    setChoiceFailed(false);
    setModalOpen(true);
  };

  const bodyOf = (source: ProductForm) => ({
    type: source.type,
    title: source.title,
    description: source.description,
    program_id: source.programId,
    price: source.price,
    original_price: source.originalPrice.trim() === "" ? null : source.originalPrice,
    is_active: source.isActive,
    active_from: source.activeFrom === "" ? null : new Date(`${source.activeFrom}T00:00:00`).toISOString(),
    active_until: source.activeUntil === "" ? null : new Date(`${source.activeUntil}T00:00:00`).toISOString(),
    tryout_ids: source.tryoutIds,
  });

  const handleSave = async () => {
    setSaving(true);
    setFormErrors({});
    try {
      if (editing) {
        const updated = await api.products.update(editing.id, bodyOf(form));
        setRows((prev) => prev?.map((row) => (row.id === updated.id ? updated : row)) ?? []);
        toast.success(t.toastUpdated);
      } else {
        const created = await api.products.create(bodyOf(form));
        setRows((prev) => [created, ...(prev ?? [])]);
        setTotal((count) => count + 1);
        toast.success(t.toastCreated);
      }
      setModalOpen(false);
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        const inline: Record<string, string> = {};
        for (const [path, message] of Object.entries(err.fields)) {
          inline[path.startsWith("tryout_ids") ? "tryout_ids" : path] = message;
        }
        setFormErrors(inline);
      }
      toast.danger(t.toastSaveFailedTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  const toggleActive = async (product: Product) => {
    try {
      const updated = await api.products.update(product.id, {
        type: product.type,
        title: product.title,
        description: product.description,
        program_id: product.program_id,
        price: product.price,
        original_price: product.original_price,
        is_active: !product.is_active,
        active_from: product.active_from,
        active_until: product.active_until,
        tryout_ids: product.tryouts.map((tryout) => tryout.id),
      });
      setRows((prev) => prev?.map((row) => (row.id === updated.id ? updated : row)) ?? []);
      toast.success(updated.is_active ? t.katalogOn : t.katalogOff);
    } catch (err) {
      toast.danger(t.toastToggleFailedTitle, { description: errorText(err, t.loadErrorDescription) });
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.products.remove(deleteTarget.id);
      setRows((prev) => prev?.filter((row) => row.id !== deleteTarget.id) ?? []);
      setTotal((count) => Math.max(0, count - 1));
      toast.success(t.toastDeleted);
      setDeleteTarget(null);
    } catch (err) {
      toast.danger(t.toastDeleteFailedTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setDeleting(false);
    }
  };

  const toggleTryout = (id: string) => {
    setForm((current) => {
      if (current.type === "individual") return { ...current, tryoutIds: [id] };
      const checked = current.tryoutIds.includes(id);
      return {
        ...current,
        tryoutIds: checked ? current.tryoutIds.filter((item) => item !== id) : [...current.tryoutIds, id],
      };
    });
  };

  const columns: Column<Product>[] = [
    { key: "title", header: t.colProduct, render: (product) => <span className="font-medium">{product.title}</span> },
    {
      key: "type",
      header: t.colType,
      render: (product) => (
        <Chip size="sm" variant="secondary" className="border border-border">
          {product.type === "bundle" ? t.typeBundle(product.tryouts.length) : t.typeIndividual}
        </Chip>
      ),
    },
    {
      key: "program",
      header: t.colProgram,
      render: (product) => {
        const program = programOf(product.program_id);
        return program ? <ProgramBadge code={program.code} /> : null;
      },
    },
    { key: "price", header: t.colPrice, render: (product) => <span className="font-medium">{formatIDR(product.price)}</span> },
    {
      key: "activeUntil",
      header: t.colActiveUntil,
      render: (product) => (product.active_until ? new Date(product.active_until).toLocaleDateString() : "—"),
    },
    {
      key: "isActive",
      header: t.colShowInCatalog,
      render: (product) => (
        <div onClick={(e) => e.stopPropagation()}>
          <Switch isSelected={product.is_active} onChange={() => toggleActive(product)}>
            <Switch.Content>
              <Switch.Control>
                <Switch.Thumb />
              </Switch.Control>
            </Switch.Content>
          </Switch>
        </div>
      ),
    },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (product) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly onPress={() => openEdit(product)}>
            <Pencil className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" onPress={() => setDeleteTarget(product)}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin", "operator"]} title={t.requireRoleTitle} deniedMessage={t.requireRoleDenied}>
      <AppShell
        title={t.pageTitle}
        description={t.productCount(total)}
        actions={
          <Button variant="primary" size="sm" onPress={openCreate}>
            <Plus className="size-3.5" /> {t.addProduct}
          </Button>
        }
      >
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
          <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
        ) : rows.length === 0 ? (
          <EmptyState icon={Package} title={t.emptyTitle} description={t.emptyDescription} />
        ) : (
          <>
            <DataTable columns={columns} rows={rows} rowKey={(row) => row.id} onRowClick={openEdit} />
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
          isOpen={modalOpen}
          onOpenChange={setModalOpen}
          title={editing ? t.editProduct : t.addProduct}
          size="lg"
          footer={
            <>
              <Button variant="ghost" onPress={() => setModalOpen(false)}>
                {t.cancel}
              </Button>
              <Button variant="primary" onPress={handleSave} isDisabled={saving}>
                {t.save}
              </Button>
            </>
          }
        >
          <div>
            <div className="flex gap-1 rounded-lg border border-border bg-surface-secondary p-1 sm:w-fit">
              {(["individual", "bundle"] as const).map((productType) => (
                <button
                  key={productType}
                  type="button"
                  onClick={() =>
                    setForm((current) => ({
                      ...current,
                      type: productType,
                      tryoutIds: productType === "individual" ? current.tryoutIds.slice(0, 1) : current.tryoutIds,
                    }))
                  }
                  className={cn(
                    "rounded-md px-4 py-1.5 text-sm font-medium transition-colors",
                    form.type === productType ? "bg-surface text-foreground shadow-surface" : "text-muted",
                  )}
                >
                  {productType === "individual" ? t.typeIndividual : t.typeBundle(form.tryoutIds.length)}
                </button>
              ))}
            </div>
            {formErrors.type && <p className="mt-1 text-xs text-danger">{formErrors.type}</p>}
          </div>

          <TextInput
            label={t.fieldTitle}
            required
            value={form.title}
            error={formErrors.title}
            onChange={(e) => setForm((current) => ({ ...current, title: e.target.value }))}
          />
          <TextareaInput
            label={t.fieldDescription}
            rows={2}
            value={form.description}
            error={formErrors.description}
            onChange={(e) => setForm((current) => ({ ...current, description: e.target.value }))}
          />
          <SelectInput
            label={t.fieldProgram}
            options={(programs ?? []).map((program) => ({ id: program.id, label: program.name }))}
            selectedKey={form.programId}
            error={formErrors.program_id}
            onSelectionChange={(key) => {
              if (!key) return;
              setForm((current) => ({ ...current, programId: key, tryoutIds: [] }));
              setChoiceTryouts(null);
              setChoiceFailed(false);
            }}
          />

          <div>
            <p className="mb-2 text-[13px] font-medium text-foreground">
              {form.type === "bundle" ? t.tryoutPickBundle : t.tryoutPickSingle}
            </p>
            <div className="space-y-1.5">
              {choiceFailed ? (
                <p className="text-xs text-danger">{t.loadErrorDescription}</p>
              ) : choiceTryouts === null ? (
                <p className="text-xs text-muted">{t.tryoutLoading}</p>
              ) : choiceTryouts.length === 0 ? (
                <p className="text-xs text-muted">{t.tryoutEmpty}</p>
              ) : (
                choiceTryouts.map((tryout) => {
                  const checked = form.tryoutIds.includes(tryout.id);
                  return (
                    <button
                      key={tryout.id}
                      type="button"
                      onClick={() => toggleTryout(tryout.id)}
                      className={cn(
                        "flex w-full items-center gap-2.5 rounded-lg border px-3 py-2 text-left text-sm transition-colors",
                        checked
                          ? "border-accent bg-accent-soft text-accent-soft-foreground"
                          : "border-border text-foreground hover:bg-surface-secondary",
                      )}
                    >
                      <span
                        className={cn(
                          "flex size-4 items-center justify-center rounded-[5px] border-2 text-[10px] font-bold",
                          checked ? "border-accent bg-accent text-white" : "border-border text-transparent",
                        )}
                      >
                        ✓
                      </span>
                      {tryout.title}
                    </button>
                  );
                })
              )}
            </div>
            {formErrors.tryout_ids && <p className="mt-1 text-xs text-danger">{formErrors.tryout_ids}</p>}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label={t.fieldPrice}
              required
              inputMode="decimal"
              value={form.price}
              error={formErrors.price}
              description={t.priceHint}
              onChange={(e) => setForm((current) => ({ ...current, price: e.target.value }))}
            />
            <TextInput
              label={t.fieldOriginalPrice}
              inputMode="decimal"
              value={form.originalPrice}
              error={formErrors.original_price}
              onChange={(e) => setForm((current) => ({ ...current, originalPrice: e.target.value }))}
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label={t.fieldActiveFrom}
              type="date"
              value={form.activeFrom}
              error={formErrors.active_from}
              onChange={(e) => setForm((current) => ({ ...current, activeFrom: e.target.value }))}
            />
            <TextInput
              label={t.fieldActiveUntil}
              type="date"
              value={form.activeUntil}
              error={formErrors.active_until}
              onChange={(e) => setForm((current) => ({ ...current, activeUntil: e.target.value }))}
            />
          </div>
          <SwitchInput
            label={t.fieldActive}
            description={t.fieldActiveDescription}
            isSelected={form.isActive}
            onChange={(checked) => setForm((current) => ({ ...current, isActive: checked }))}
          />
        </FormModal>

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
                {t.deleteConfirm}
              </Button>
            </>
          }
        >
          <p className="text-sm text-muted">{t.deleteDescription}</p>
        </FormModal>
      </AppShell>
    </RequireRole>
  );
}
