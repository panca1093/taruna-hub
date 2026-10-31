import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { Icon } from "./icon";

/** One number with caption (prototype .metric). Use inside MetricStrip or alone. */
export function Metric({
  value,
  label,
  icon,
  note,
  size = "md",
  className,
}: {
  value: string;
  label: string;
  icon?: LucideIcon;
  note?: string;
  size?: "md" | "lg" | "xl";
  className?: string;
}) {
  const valueClass = size === "xl" ? "text-metric-xl" : size === "lg" ? "text-metric-lg" : "text-metric";
  return (
    <div className={cn("flex min-w-0 items-center gap-4 px-3 sm:px-6", className)}>
      {icon && (
        <span className="hidden size-9.5 shrink-0 place-items-center rounded-control bg-background text-rose sm:grid">
          <Icon icon={icon} size="sm" />
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
