import { Card } from "@heroui/react";
import type { LucideIcon } from "lucide-react";
import { ArrowDownRight, ArrowUpRight } from "lucide-react";
import { cn } from "@/lib/cn";
import { CountdownRing } from "./countdown-ring";

interface StatCardProps {
  label: string;
  value: string;
  icon: LucideIcon;
  delta?: { value: string; direction: "up" | "down" };
  ring?: number;
  className?: string;
  compact?: boolean;
}

export function StatCard({ label, value, icon: Icon, delta, ring, className, compact }: StatCardProps) {
  if (compact) {
    return (
      <Card className={cn("relative gap-2 rounded-2xl border border-separator p-5 shadow-none", className)}>
        <p className="pr-6 text-xs leading-relaxed text-muted">{label}</p>
        <Icon className="absolute right-5 top-5 size-4 text-muted" aria-hidden="true" />
        <p className="break-words font-display text-2xl font-semibold tracking-tight text-foreground tabular-nums">{value}</p>
      </Card>
    );
  }
  return (
    <Card className={cn("gap-4 p-5", className)}>
      <div className="flex items-start justify-between">
        <span className="inline-flex size-9 items-center justify-center rounded-lg bg-accent-soft text-accent-soft-foreground">
          <Icon className="size-[18px]" strokeWidth={2} />
        </span>
        {ring !== undefined && (
          <CountdownRing value={ring} size={36} strokeWidth={4} animate={false}>
            <span className="font-mono text-[10px] font-medium text-muted">{Math.round(ring)}%</span>
          </CountdownRing>
        )}
      </div>
      <div>
        <p className="font-display text-2xl font-semibold tracking-tight text-foreground">{value}</p>
        <p className="mt-0.5 text-sm text-muted">{label}</p>
      </div>
      {delta && (
        <div
          className={cn(
            "inline-flex w-fit items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium",
            delta.direction === "up" ? "bg-success-soft text-success-soft-foreground" : "bg-danger-soft text-danger-soft-foreground",
          )}
        >
          {delta.direction === "up" ? <ArrowUpRight className="size-3" /> : <ArrowDownRight className="size-3" />}
          {delta.value}
        </div>
      )}
    </Card>
  );
}
