import { EmptyState as HeroEmptyState } from "@heroui/react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <HeroEmptyState className={cn("flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-14 text-center", className)}>
      <span className="inline-flex size-11 items-center justify-center rounded-full bg-surface-secondary text-muted">
        <Icon className="size-5" strokeWidth={1.75} />
      </span>
      <div className="space-y-1">
        <p className="text-sm font-medium text-foreground">{title}</p>
        {description && <p className="mx-auto max-w-sm text-sm text-muted">{description}</p>}
      </div>
      {action}
    </HeroEmptyState>
  );
}
