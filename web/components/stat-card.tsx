import { Card } from "@heroui/react";
import type { LucideIcon } from "lucide-react";
import { ArrowDownRight, ArrowUpRight } from "lucide-react";
import { cn } from "@/lib/cn";
import { CountdownRing } from "./countdown-ring";
import { Icon } from "./icon";

interface StatCardProps {
  label: string;
  value: string;
  icon: LucideIcon;
  delta?: { value: string; direction: "up" | "down" };
  ring?: number;
  className?: string;
  compact?: boolean;
}

export function StatCard({ label, value, icon, delta, ring, className, compact }: StatCardProps) {
  if (compact) {
    return (
      <Card className={cn("relative gap-2 rounded-tile p-5 shadow-none", className)}>
        <p className="pr-6 text-micro leading-relaxed text-muted">{label}</p>
        <Icon icon={icon} size="sm" className="absolute right-5 top-5 text-rose" />
        <p className="break-words font-display text-metric tabular-nums text-foreground">{value}</p>
      </Card>
    );
  }
  return (
    <Card className={cn("gap-4 rounded-tile p-5 shadow-soft", className)}>
      <div className="flex items-start justify-between">
        <span className="inline-flex size-9.5 items-center justify-center rounded-control bg-background text-rose">
          <Icon icon={icon} size="sm" />
        </span>
        {ring !== undefined && (
          <CountdownRing value={ring} size={36} strokeWidth={4} animate={false}>
            <span className="font-mono text-micro font-medium text-muted">{Math.round(ring)}%</span>
          </CountdownRing>
        )}
      </div>
      <div>
        <p className="font-display text-metric-lg tabular-nums text-foreground">{value}</p>
        <p className="mt-0.5 text-label text-muted">{label}</p>
      </div>
      {delta && (
        <div
          className={cn(
            "inline-flex w-fit items-center gap-1 rounded-chip px-2 py-1 text-micro font-medium",
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
