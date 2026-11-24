"use client";

import { useState } from "react";
import { Button, Chip } from "@heroui/react";
import { X } from "lucide-react";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { useProductItemPickerT } from "@/lib/i18n/dict/product-item-picker";

export type PickerKind = "tryout" | "kecermatan";
export interface PickerItem {
  id: string;
  label: string;
  kind: PickerKind;
}


/**
 * Selected items as a compact list plus a button that opens a second modal with search and a draft selection.
 * `multi` shows a Tryout / Kecermatan switch over the same search and enforces `min`..`max` in total.
 */
export function ProductItemPicker({
  multi,
  label,
  buttonLabel,
  title,
  items,
  selectedIds,
  onChange,
  loading,
  failed,
  loadingText,
  failedText,
  emptyText,
  error,
  min = 1,
  max = 1,
  isDisabled,
}: {
  multi: boolean;
  label: string;
  buttonLabel: string;
  title: string;
  items: PickerItem[];
  selectedIds: string[];
  onChange: (ids: string[]) => void;
  loading: boolean;
  failed: boolean;
  loadingText: string;
  failedText: string;
  emptyText: string;
  error?: string;
  min?: number;
  max?: number;
  isDisabled?: boolean;
}) {
  const c = useProductItemPickerT();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<string[]>([]);
  const [query, setQuery] = useState("");
  const [tab, setTab] = useState<PickerKind>("tryout");

  const byId = new Map(items.map((item) => [item.id, item]));
  const picked = selectedIds.map((id) => byId.get(id) ?? { id, label: c.unavailable, kind: null });
  const kindLabel = (kind: PickerKind) => (kind === "tryout" ? c.tryout : c.kecermatan);

  const openPicker = () => {
    setDraft(selectedIds);
    setQuery("");
    setTab(items.some((item) => item.kind === "tryout") || !multi ? "tryout" : "kecermatan");
    setOpen(true);
  };

  const countOf = (kind: PickerKind) => items.filter((item) => item.kind === kind).length;
  const needle = query.trim().toLowerCase();
  const visible = items.filter(
    (item) => (!multi || item.kind === tab) && (needle === "" || item.label.toLowerCase().includes(needle)),
  );

  const toggle = (id: string) =>
    setDraft((current) => {
      if (!multi) return [id];
      if (current.includes(id)) return current.filter((item) => item !== id);
      return current.length >= max ? current : [...current, id];
    });

  const valid = multi ? draft.length >= min && draft.length <= max : draft.length === 1;

  return (
    <div>
      <p className="mb-2 text-small font-medium text-foreground">
        {label}
        {multi && selectedIds.length > 0 && <span className="ml-1.5 text-muted">· {c.selected(selectedIds.length)}</span>}
      </p>
      {picked.length > 0 && (
        <ul className="mb-2 flex flex-col gap-1.5">
          {picked.map((item) => (
            <li key={item.id} className="flex items-center gap-2 rounded-control border border-border px-3 py-2 text-small text-foreground">
              <span className={cn("min-w-0 flex-1 truncate", item.kind === null && "text-danger")} title={item.label}>
                {item.label}
              </span>
              {multi && item.kind !== null && (
                <Chip size="sm" variant="secondary" className="border border-border font-medium">
                  {kindLabel(item.kind)}
                </Chip>
              )}
              <button
                type="button"
                aria-label={c.remove(item.label)}
                onClick={() => onChange(selectedIds.filter((id) => id !== item.id))}
                className="rounded-chip p-1 text-muted hover:bg-surface-secondary hover:text-foreground"
              >
                <X className="size-3.5" aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}
      {failed ? (
        <p className="text-label text-danger">{failedText}</p>
      ) : loading ? (
        <p className="text-label text-muted">{loadingText}</p>
      ) : (
        <>
          <Button variant="secondary" size="sm" onPress={openPicker} isDisabled={isDisabled}>
            {buttonLabel}
          </Button>
          {items.length === 0 && <p className="mt-1.5 text-label text-muted">{emptyText}</p>}
        </>
      )}
      {error && <p className="mt-1 text-label text-danger">{error}</p>}

      <FormModal
        isOpen={open}
        onOpenChange={setOpen}
        title={title}
        description={multi ? c.rule(min, max) : undefined}
        size="md"
        footer={
          <>
            <Button variant="ghost" onPress={() => setOpen(false)}>
              {c.cancel}
            </Button>
            <Button
              variant="primary"
              isDisabled={!valid}
              onPress={() => {
                onChange(draft);
                setOpen(false);
              }}
            >
              {multi ? c.apply : c.choose}
            </Button>
          </>
        }
      >
        <TextInput aria-label={c.search} placeholder={c.search} value={query} onChange={(e) => setQuery(e.target.value)} />
        {multi && (
          <div role="tablist" className="flex gap-1 rounded-control border border-border bg-surface-secondary p-1 sm:w-fit">
            {(["tryout", "kecermatan"] as const).map((kind) => (
              <button
                key={kind}
                type="button"
                role="tab"
                aria-selected={tab === kind}
                onClick={() => setTab(kind)}
                className={cn(
                  "rounded-chip px-4 py-1.5 text-small font-medium transition-colors",
                  tab === kind ? "bg-surface text-foreground shadow-surface" : "text-muted",
                )}
              >
                {kindLabel(kind)} ({countOf(kind)})
              </button>
            ))}
          </div>
        )}
        <div role={multi ? "group" : "radiogroup"} className="flex max-h-72 flex-col gap-1.5 overflow-y-auto">
          {visible.length === 0 ? (
            <p className="text-label text-muted">{needle === "" ? emptyText : c.noMatch}</p>
          ) : (
            visible.map((item) => {
              const checked = draft.includes(item.id);
              return (
                <button
                  key={item.id}
                  type="button"
                  role={multi ? "checkbox" : "radio"}
                  aria-checked={checked}
                  onClick={() => toggle(item.id)}
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-control border px-3 py-2 text-left text-small transition-colors",
                    checked
                      ? "border-accent bg-accent-soft text-accent-soft-foreground"
                      : "border-border text-foreground hover:bg-surface-secondary",
                  )}
                >
                  <span
                    className={cn(
                      "flex size-4 shrink-0 items-center justify-center border-2 text-micro font-bold",
                      multi ? "rounded-chip" : "rounded-full",
                      checked ? "border-accent bg-accent text-accent-foreground" : "border-border text-transparent",
                    )}
                  >
                    ✓
                  </span>
                  <span className="truncate" title={item.label}>
                    {item.label}
                  </span>
                </button>
              );
            })
          )}
        </div>
        {multi && (
          <p className={cn("text-label", valid ? "text-muted" : "text-danger")}>
            {c.selected(draft.length)} / {min}–{max}
          </p>
        )}
      </FormModal>
    </div>
  );
}
