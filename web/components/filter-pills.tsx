"use client";

import { useRef } from "react";
import { cn } from "@/lib/cn";

export interface PillOption {
  id: string;
  label: string;
  count?: number;
}

/** Rounded filter chips (prototype .filter). Single-select; the parent owns the value. */
export function FilterPills({ options, value, onChange, ariaLabel, className }: { options: PillOption[]; value: string; onChange: (id: string) => void; ariaLabel: string; className?: string }) {
  return (
    <div role="group" aria-label={ariaLabel} className={cn("flex flex-wrap items-center gap-2", className)}>
      {options.map((option) => {
        const selected = option.id === value;
        return (
          <button
            key={option.id}
            type="button"
            aria-pressed={selected}
            onClick={() => onChange(option.id)}
            className={cn(
              "min-h-9 rounded-full border px-3.5 text-label outline-none transition-colors focus-visible:ring-2 focus-visible:ring-focus",
              selected ? "border-accent bg-accent text-accent-foreground" : "border-border bg-surface text-muted hover:text-foreground",
            )}
          >
            {option.label}
            {option.count !== undefined && <span className="ml-1.5 tabular-nums opacity-80">{option.count}</span>}
          </button>
        );
      })}
    </div>
  );
}

export const tabId = (idPrefix: string, id: string) => `${idPrefix}-tab-${id}`;
export const tabPanelId = (idPrefix: string) => `${idPrefix}-panel`;

/** Underlined tab strip with a count badge (prototype .hub-tabs). Controlled; render one panel yourself with `id={tabPanelId(idPrefix)}`, `role="tabpanel"` and `aria-labelledby={tabId(idPrefix, value)}`. */
export function UnderlineTabs({
  options,
  value,
  onChange,
  ariaLabel,
  idPrefix = "tabs",
  className,
}: {
  options: PillOption[];
  value: string;
  onChange: (id: string) => void;
  ariaLabel: string;
  idPrefix?: string;
  className?: string;
}) {
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});
  const selectedId = options.some((option) => option.id === value) ? value : options[0]?.id;

  const onKeyDown = (event: React.KeyboardEvent, index: number) => {
    const last = options.length - 1;
    const target = event.key === "ArrowRight" ? (index === last ? 0 : index + 1) : event.key === "ArrowLeft" ? (index === 0 ? last : index - 1) : event.key === "Home" ? 0 : event.key === "End" ? last : null;
    if (target === null) return;
    event.preventDefault();
    const next = options[target];
    onChange(next.id);
    refs.current[next.id]?.focus();
  };

  return (
    <div role="tablist" aria-label={ariaLabel} className={cn("mb-6 flex gap-1.5 overflow-x-auto border-b border-border", className)}>
      {options.map((option, index) => {
        const selected = option.id === selectedId;
        return (
          <button
            key={option.id}
            ref={(node) => {
              refs.current[option.id] = node;
            }}
            id={tabId(idPrefix, option.id)}
            type="button"
            role="tab"
            aria-selected={selected}
            aria-controls={tabPanelId(idPrefix)}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(option.id)}
            onKeyDown={(event) => onKeyDown(event, index)}
            className={cn(
              "relative whitespace-nowrap px-5 py-3.5 text-small outline-none focus-visible:ring-2 focus-visible:ring-focus",
              selected ? "font-semibold text-accent after:absolute after:inset-x-3.5 after:-bottom-px after:h-0.75 after:rounded-full after:bg-accent" : "text-muted hover:text-foreground",
            )}
          >
            {option.label}
            {option.count !== undefined && <span className="ml-2 rounded-chip bg-accent-soft px-1.5 py-0.5 text-micro">{option.count}</span>}
          </button>
        );
      })}
    </div>
  );
}
