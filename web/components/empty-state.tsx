import { EmptyState as HeroEmptyState } from "@heroui/react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { Icon } from "./icon";

/**
 * Prototype empty screen: rose icon, Geist title, muted copy, optional action.
 * Draws its own card by default; pass `bare` when it already sits inside a panel.
 */
export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
  bare,
}: {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
  bare?: boolean;
}) {
  return (
    <HeroEmptyState
      className={cn("flex flex-col items-center gap-3 px-5 py-12 text-center", !bare && "rounded-card border border-border bg-surface", className)}
    >
      <Icon icon={icon} size="hero" className="text-rose" />
      <div className="space-y-2">
        <p className="font-display text-title text-foreground">{title}</p>
        {description && <p className="mx-auto max-w-sm text-caption leading-relaxed text-muted">{description}</p>}
      </div>
      {action}
    </HeroEmptyState>
  );
}
