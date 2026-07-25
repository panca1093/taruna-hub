"use client";

import { useState } from "react";
import { Button, Chip, Switch, toast } from "@heroui/react";
import { Package, Pencil, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { ProgramBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput, TextareaInput } from "@/components/ui/form";
import { useProductsT } from "@/lib/i18n/dict/products";
import {
  formatDate,
  formatIDR,
  getProgram,
  kecermatanTests,
  programs,
  products as initialProducts,
  tryouts,
} from "@/lib/mock-data";
import type { Product, ProductType } from "@/lib/types";
import { cn } from "@/lib/cn";

const programOptions = programs.map((p) => ({ id: p.id, label: p.name }));

const covers = [
  ["#4f46e5", "#8b5cf6"],
  ["#38bdf8", "#6366f1"],
  ["#f59e0b", "#ef4444"],
  ["#10b981", "#0ea5e9"],
  ["#8b5cf6", "#d946ef"],
  ["#0ea5e9", "#4f46e5"],
] as const;

interface ProductForm {
  type: ProductType;
  title: string;
  description: string;
  programId: string;
  tryoutIds: string[];
  kecermatanTestId: string;
  includedAttempts: string;
  price: string;
  originalPrice: string;
  activeUntil: string;
  isActive: boolean;
  coverFrom: string;
  coverTo: string;
}

const blankForm: ProductForm = {
  type: "individual",
  title: "",
  description: "",
  programId: programs[0].id,
  tryoutIds: [],
  kecermatanTestId: "",
  includedAttempts: "3",
  price: "",
  originalPrice: "",
  activeUntil: "",
  isActive: true,
  coverFrom: covers[0][0],
  coverTo: covers[0][1],
};

export default function ProductsPage() {
  const t = useProductsT();
  const [rows, setRows] = useState<Product[]>(initialProducts);
  const [editing, setEditing] = useState<Product | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Product | null>(null);
  const [form, setForm] = useState<ProductForm>(blankForm);

  const openCreate = () => {
    setEditing(null);
    setForm(blankForm);
    setModalOpen(true);
  };

  const openEdit = (p: Product) => {
    setEditing(p);
    setForm({
      type: p.type,
      title: p.title,
      description: p.description,
      programId: p.programId,
      tryoutIds: p.tryoutIds,
      kecermatanTestId: p.kecermatanTestId ?? "",
      includedAttempts: String(p.includedAttempts ?? 3),
      price: String(p.price),
      originalPrice: String(p.originalPrice),
      activeUntil: p.activeUntil,
      isActive: p.isActive,
      coverFrom: p.coverFrom,
      coverTo: p.coverTo,
    });
    setModalOpen(true);
  };

  const toggleActive = (id: string) => {
    setRows((prev) => prev.map((p) => (p.id === id ? { ...p, isActive: !p.isActive } : p)));
  };

  const handleSave = () => {
    const isKecermatan = form.type === "kecermatan";
    if (!form.title || !form.price || (isKecermatan ? !form.kecermatanTestId : form.tryoutIds.length === 0)) {
      toast.danger(t.incompleteTitle, {
        description: isKecermatan ? t.incompleteKecermatan : t.incompleteDefault,
      });
      return;
    }
    const payload = {
      type: form.type,
      title: form.title,
      description: form.description,
      programId: form.programId,
      tryoutIds: isKecermatan ? [] : form.tryoutIds,
      kecermatanTestId: isKecermatan ? form.kecermatanTestId : undefined,
      includedAttempts: isKecermatan ? Number(form.includedAttempts) || 1 : undefined,
      price: Number(form.price),
      originalPrice: Number(form.originalPrice) || Number(form.price),
      activeUntil: form.activeUntil,
      isActive: form.isActive,
      coverFrom: form.coverFrom,
      coverTo: form.coverTo,
    };
    if (editing) {
      setRows((prev) => prev.map((p) => (p.id === editing.id ? { ...p, ...payload } : p)));
      toast.success(t.productUpdated);
    } else {
      setRows((prev) => [{ id: `prod-new-${Date.now()}`, ...payload }, ...prev]);
      toast.success(t.productAdded);
    }
    setModalOpen(false);
  };

  const columns: Column<Product>[] = [
    {
      key: "title",
      header: t.colProduct,
      render: (p) => (
        <div className="flex items-center gap-3">
          <div className="size-8 shrink-0 rounded-md" style={{ background: `linear-gradient(135deg, ${p.coverFrom}, ${p.coverTo})` }} />
          <span className="font-medium">{p.title}</span>
        </div>
      ),
    },
    {
      key: "type",
      header: t.colType,
      render: (p) => (
        <Chip size="sm" variant="secondary" className="border border-border">
          {p.type === "bundle" ? t.typeBundle(p.tryoutIds.length) : p.type === "kecermatan" ? t.typeKecermatan : t.typeIndividual}
        </Chip>
      ),
    },
    { key: "program", header: t.colProgram, render: (p) => { const pr = getProgram(p.programId); return pr ? <ProgramBadge code={pr.code} /> : null; } },
    { key: "price", header: t.colPrice, render: (p) => <span className="font-medium">{formatIDR(p.price)}</span> },
    { key: "activeUntil", header: t.colActiveUntil, render: (p) => formatDate(p.activeUntil) },
    {
      key: "isActive",
      header: t.colShowInCatalog,
      render: (p) => (
        <div onClick={(e) => e.stopPropagation()}>
          <Switch isSelected={p.isActive} onChange={() => toggleActive(p.id)}>
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
      render: (p) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly onPress={() => openEdit(p)}>
            <Pencil className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" onPress={() => setDeleteTarget(p)}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <AppShell
      title={t.pageTitle}
      description={t.productCount(rows.length)}
      actions={
        <Button variant="primary" size="sm" onPress={openCreate}>
          <Plus className="size-3.5" /> {t.addProduct}
        </Button>
      }
    >
      {rows.length === 0 ? (
        <EmptyState icon={Package} title={t.emptyTitle} />
      ) : (
        <DataTable columns={columns} rows={rows} rowKey={(p) => p.id} onRowClick={openEdit} />
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
            <Button variant="primary" onPress={handleSave}>
              {t.save}
            </Button>
          </>
        }
      >
        <div className="flex gap-1 rounded-lg border border-border bg-surface-secondary p-1 sm:w-fit">
          {(["individual", "bundle", "kecermatan"] as ProductType[]).map((pt) => (
            <button
              key={pt}
              type="button"
              onClick={() =>
                setForm((f) => ({
                  ...f,
                  type: pt,
                  tryoutIds: pt === "individual" ? f.tryoutIds.slice(0, 1) : f.tryoutIds,
                }))
              }
              className={cn(
                "rounded-md px-4 py-1.5 text-sm font-medium transition-colors",
                form.type === pt ? "bg-surface text-foreground shadow-surface" : "text-muted",
              )}
            >
              {pt === "individual" ? t.typeIndividual : pt === "bundle" ? "Bundle" : t.typeKecermatan}
            </button>
          ))}
        </div>

        <TextInput label={t.fieldTitle} required value={form.title} onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} />
        <TextareaInput label={t.fieldDescription} rows={2} value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} />
        <SelectInput
          label={t.fieldProgram}
          options={programOptions}
          selectedKey={form.programId}
          onSelectionChange={(k) => k && setForm((f) => ({ ...f, programId: k, tryoutIds: [], kecermatanTestId: "" }))}
        />

        {form.type === "kecermatan" ? (
          <div>
            <p className="mb-2 text-[13px] font-medium text-foreground">{t.kecermatanPickTitle}</p>
            <div className="space-y-1.5">
              {kecermatanTests
                .filter((t) => t.programId === form.programId)
                .map((t) => {
                  const checked = form.kecermatanTestId === t.id;
                  return (
                    <button
                      key={t.id}
                      type="button"
                      onClick={() => setForm((f) => ({ ...f, kecermatanTestId: t.id }))}
                      className={cn(
                        "flex w-full items-center gap-2.5 rounded-lg border px-3 py-2 text-left text-sm transition-colors",
                        checked ? "border-accent bg-accent-soft text-accent-soft-foreground" : "border-border text-foreground hover:bg-surface-secondary",
                      )}
                    >
                      <span className={cn("flex size-4 items-center justify-center rounded-[5px] border-2 text-[10px] font-bold", checked ? "border-accent bg-accent text-white" : "border-border text-transparent")}>✓</span>
                      {t.title}
                    </button>
                  );
                })}
              {kecermatanTests.filter((t) => t.programId === form.programId).length === 0 && (
                <p className="text-xs text-muted">{t.kecermatanEmpty}</p>
              )}
            </div>
          </div>
        ) : (
          <div>
            <p className="mb-2 text-[13px] font-medium text-foreground">
              {form.type === "bundle" ? t.tryoutPickBundle : t.tryoutPickSingle}
            </p>
            <div className="space-y-1.5">
              {tryouts
                .filter((t) => t.programId === form.programId)
                .map((t) => {
                  const checked = form.tryoutIds.includes(t.id);
                  return (
                    <button
                      key={t.id}
                      type="button"
                      onClick={() =>
                        setForm((f) => ({
                          ...f,
                          tryoutIds:
                            f.type === "individual"
                              ? [t.id]
                              : checked
                                ? f.tryoutIds.filter((id) => id !== t.id)
                                : [...f.tryoutIds, t.id],
                        }))
                      }
                      className={cn(
                        "flex w-full items-center gap-2.5 rounded-lg border px-3 py-2 text-left text-sm transition-colors",
                        checked ? "border-accent bg-accent-soft text-accent-soft-foreground" : "border-border text-foreground hover:bg-surface-secondary",
                      )}
                    >
                      <span className={cn("flex size-4 items-center justify-center rounded-[5px] border-2 text-[10px] font-bold", checked ? "border-accent bg-accent text-white" : "border-border text-transparent")}>✓</span>
                      {t.title}
                    </button>
                  );
                })}
              {tryouts.filter((t) => t.programId === form.programId).length === 0 && (
                <p className="text-xs text-muted">{t.tryoutEmpty}</p>
              )}
            </div>
          </div>
        )}

        <div className="grid gap-4 sm:grid-cols-2">
          <TextInput label={t.fieldPrice} type="number" required value={form.price} onChange={(e) => setForm((f) => ({ ...f, price: e.target.value }))} />
          <TextInput label={t.fieldOriginalPrice} type="number" value={form.originalPrice} onChange={(e) => setForm((f) => ({ ...f, originalPrice: e.target.value }))} />
        </div>
        {form.type === "kecermatan" && (
          <TextInput
            label={t.fieldIncludedAttempts}
            type="number"
            min={1}
            required
            value={form.includedAttempts}
            onChange={(e) => setForm((f) => ({ ...f, includedAttempts: e.target.value }))}
          />
        )}
        <TextInput label={t.fieldActiveUntil} type="date" required value={form.activeUntil} onChange={(e) => setForm((f) => ({ ...f, activeUntil: e.target.value }))} />

        <div>
          <p className="mb-2 text-[13px] font-medium text-foreground">{t.coverColorLabel}</p>
          <div className="flex gap-2">
            {covers.map(([from, to]) => (
              <button
                key={from + to}
                type="button"
                onClick={() => setForm((f) => ({ ...f, coverFrom: from, coverTo: to }))}
                className={cn(
                  "size-8 rounded-md ring-offset-2 ring-offset-surface transition-shadow",
                  form.coverFrom === from && form.coverTo === to && "ring-2 ring-accent",
                )}
                style={{ background: `linear-gradient(135deg, ${from}, ${to})` }}
                aria-label={t.coverColorAria}
              />
            ))}
          </div>
        </div>
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
            <Button
              variant="danger"
              onPress={() => {
                setRows((prev) => prev.filter((p) => p.id !== deleteTarget?.id));
                toast.success(t.productDeleted);
                setDeleteTarget(null);
              }}
            >
              {t.deleteConfirm}
            </Button>
          </>
        }
      >
        <p className="text-sm text-muted">{t.deleteDescription}</p>
      </FormModal>
    </AppShell>
  );
}
