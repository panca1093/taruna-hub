import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { Icon } from "./icon";

const toneClass = {
  neutral: "bg-background text-rose",
  blue: "bg-viz-blue-soft text-viz-blue",
  green: "bg-viz-green-soft text-viz-green",
  amber: "bg-viz-amber-soft text-viz-amber",
  rose: "bg-accent-soft text-accent",
} as const;

/** One number with caption (prototype .metric). Use inside MetricStrip or alone. */
export function Metric({
  value,
  label,
  icon,
  note,
  size = "md",
  tone = "neutral",
  className,
}: {
  value: string;
  label: string;
  icon?: LucideIcon;
  note?: string;
  size?: "md" | "lg" | "xl";
  tone?: keyof typeof toneClass;
  className?: string;
}) {
  const valueClass = size === "xl" ? "text-metric-xl" : size === "lg" ? "text-metric-lg" : "text-metric";
  return (
    <div className={cn("flex min-w-0 items-center gap-4 px-3 sm:px-6", className)}>
      {icon && (
        <span className={cn("hidden size-11 shrink-0 place-items-center rounded-control sm:grid", toneClass[tone])}>
          <Icon icon={icon} size="md" />
        </span>
      )}
      <div className="min-w-0">
        <p className={cn("font-display tabular-nums", valueClass)}>{value}</p>
        <p className="mt-0.5 text-micro text-muted">{label}</p>
        {note && <p className="mt-0.5 hidden text-micro text-muted sm:block">{note}</p>}
      </div>
    </div>
  );
}

/** Row of 2-4 metrics in one bordered card with hairline dividers (prototype .metric-strip). */
export function MetricStrip({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={cn("grid grid-cols-2 gap-y-4 rounded-tile border border-border bg-surface py-4 shadow-soft sm:auto-cols-fr sm:grid-flow-col sm:gap-y-0 max-sm:[&>*:nth-child(even)]:border-l max-sm:[&>*:nth-child(n+3)]:border-t max-sm:[&>*:nth-child(n+3)]:pt-4 sm:[&>*+*]:border-l [&>*]:border-border", className)}>
      {children}
    </div>
  );
}
