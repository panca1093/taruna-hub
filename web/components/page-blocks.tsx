import Link from "next/link";
import { ArrowRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { Icon } from "./icon";

/** Small accent label with a leading icon at the top of a card (prototype .card-kicker). */
export function CardKicker({ icon, children, className }: { icon?: LucideIcon; children: React.ReactNode; className?: string }) {
  return (
    <p className={cn("mb-7 flex items-center gap-2 text-label text-accent", className)}>
      {icon && <Icon icon={icon} size="sm" />}
      {children}
    </p>
  );
}

const panelTone = {
  plain: "bg-surface border border-border",
  peach: "bg-peach",
  soft: "bg-accent-soft",
  canvas: "bg-background border border-border",
} as const;

/** Card surface. `peach` and `soft` are the tinted hero cards; `elevated` adds the soft shadow. */
export function Panel({
  tone = "plain",
  elevated,
  radius = "card",
  as: Tag = "div",
  className,
  children,
}: {
  tone?: keyof typeof panelTone;
  elevated?: boolean;
  radius?: "tile" | "card" | "hero";
  as?: "div" | "section" | "article" | "aside";
  className?: string;
  children: React.ReactNode;
}) {
  const rounded = radius === "tile" ? "rounded-tile" : radius === "hero" ? "rounded-hero" : "rounded-card";
  return <Tag className={cn("min-w-0 p-6 sm:p-7", rounded, panelTone[tone], elevated && "shadow-soft", className)}>{children}</Tag>;
}

/** Section title with an optional link on the right (prototype .section-heading). */
export function SectionHeader({
  title,
  description,
  href,
  linkLabel,
  actions,
  className,
}: {
  title: string;
  description?: string;
  href?: string;
  linkLabel?: string;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("mb-4 flex items-center justify-between gap-4", className)}>
      <div className="min-w-0">
        <h2 className="font-display text-heading-sm">{title}</h2>
        {description && <p className="mt-1 text-label text-muted">{description}</p>}
      </div>
      {actions}
      {href && linkLabel && <TextLink href={href}>{linkLabel}</TextLink>}
    </div>
  );
}

/** Accent text link with a trailing arrow (prototype .text-button). */
export function TextLink({ href, children, className, arrow = true }: { href: string; children: React.ReactNode; className?: string; arrow?: boolean }) {
  return (
    <Link
      href={href}
      className={cn("inline-flex min-h-9 items-center gap-1.5 rounded-chip text-caption font-medium text-accent outline-none hover:underline focus-visible:ring-2 focus-visible:ring-focus", className)}
    >
      {children}
      {arrow && <Icon icon={ArrowRight} size="xs" />}
    </Link>
  );
}

/** Quiet bordered note; `success` tint for positive confirmations (prototype .note-box). */
export function NoteBox({ tone = "neutral", icon, children, className }: { tone?: "neutral" | "success"; icon?: LucideIcon; children: React.ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "flex items-start gap-2.5 rounded-control p-4 text-label leading-relaxed",
        tone === "success" ? "bg-success-soft text-success-soft-foreground" : "border border-border bg-background text-muted",
        className,
      )}
    >
      {icon && <Icon icon={icon} size="sm" className="mt-0.5" />}
      <div className="min-w-0">{children}</div>
    </div>
  );
}

/** Thin progress bar for score breakdowns (prototype .score-bars). `value` is 0-100. */
export function ScoreBar({ label, valueLabel, value, className }: { label: string; valueLabel?: string; value: number; className?: string }) {
  const clamped = Math.min(100, Math.max(0, value));
  return (
    <div className={className}>
      <div className="mb-2 flex justify-between gap-3 text-caption">
        <span>{label}</span>
        {valueLabel && <span className="tabular-nums text-muted">{valueLabel}</span>}
      </div>
      <div className="h-1.5 overflow-hidden rounded-chip bg-background" role="progressbar" aria-label={label} aria-valuenow={Math.round(clamped)} aria-valuemin={0} aria-valuemax={100}>
        <div className="h-full rounded-chip bg-accent" style={{ width: `${clamped}%` }} />
      </div>
    </div>
  );
}
