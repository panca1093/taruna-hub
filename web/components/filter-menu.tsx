"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@heroui/react";
import { Search, SlidersHorizontal } from "lucide-react";
import { SelectInput, TextInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { useFilterMenuT } from "@/lib/i18n/dict/filter-menu";

/** One control in the panel. A select's "all" choice and an empty date both mean "no filter" (stored as ""). */
export type FilterField =
  | { key: string; label: string; kind?: "select"; options: { id: string; label: string }[]; allLabel?: string; half?: boolean }
  | { key: string; label: string; kind: "date"; half?: boolean };

export type FilterValue = Record<string, string>;

const ALL = "all";

/** The list-page filter (DESIGN §6.4): a button beside the search field opening an anchored panel of labelled controls. */
export function FilterMenu({
  fields,
  value,
  onChange,
  align = "end",
}: {
  fields: FilterField[];
  value: FilterValue;
  onChange: (value: FilterValue) => void;
  align?: "start" | "end";
}) {
  const t = useFilterMenuT();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value);
  const root = useRef<HTMLDivElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  const close = (restoreFocus: boolean) => {
    setOpen(false);
    if (restoreFocus) trigger.current?.focus();
  };

  useEffect(() => {
    if (open) panel.current?.querySelector<HTMLElement>("input, button")?.focus();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    // An open select renders its list outside this panel; its trigger inside the panel reports aria-expanded.
    const selectOpen = () => !!panel.current?.querySelector('[aria-expanded="true"]');
    const onPointer = (event: PointerEvent) => {
      if (selectOpen()) return;
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !selectOpen()) close(true);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const active = fields.filter((field) => value[field.key]).length;
  const set = (key: string, next: string) => setDraft((current) => ({ ...current, [key]: next }));

  return (
    <div ref={root} className="relative shrink-0">
      <Button
        ref={trigger}
        variant="secondary"
        aria-expanded={open}
        aria-haspopup="dialog"
        onPress={() => {
          setDraft(value);
          setOpen((current) => !current);
        }}
      >
        <SlidersHorizontal className="size-4" /> {t.filter}
        {active > 0 && <span className="flex size-5 items-center justify-center rounded-full bg-accent text-micro font-semibold text-accent-foreground">{active}</span>}
      </Button>
      {open && (
        <div
          ref={panel}
          role="dialog"
          aria-label={t.filter}
          className={cn("absolute top-full z-30 mt-2 w-80 max-w-[calc(100vw-2rem)] rounded-menu border border-border bg-overlay p-4 shadow-menu", align === "end" ? "right-0" : "right-0 sm:right-auto sm:left-0")}
        >
          <div className="grid grid-cols-2 gap-3">
            {fields.map((field) => (
              <div key={field.key} className={field.half ? undefined : "col-span-2"}>
                {field.kind === "date" ? (
                  <TextInput label={field.label} type="date" value={draft[field.key] ?? ""} onChange={(e) => set(field.key, e.target.value)} />
                ) : (
                  <SelectInput
                    label={field.label}
                    options={[{ id: ALL, label: field.allLabel ?? t.all }, ...field.options]}
                    selectedKey={draft[field.key] || ALL}
                    onSelectionChange={(key) => set(field.key, key === null || key === ALL ? "" : key)}
                  />
                )}
              </div>
            ))}
          </div>
          <div className="mt-4 flex items-center gap-2">
            <button type="button" onClick={() => setDraft({})} className="mr-auto text-caption font-medium text-muted hover:text-foreground">
              {t.reset}
            </button>
            <Button variant="ghost" size="sm" onPress={() => close(true)}>
              {t.cancel}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onPress={() => {
                onChange(draft);
                close(true);
              }}
            >
              {t.apply}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

/** The rounded search field that sits left of FilterMenu on every list page. */
export function SearchField({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <div className="relative min-w-0 flex-1 sm:max-w-sm">
      <Search className="absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted" />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className="w-full rounded-full border border-border bg-field py-2 pr-4 pl-10 text-small text-field-foreground outline-none focus:border-accent"
      />
    </div>
  );
}
