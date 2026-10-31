import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";

/** Icon sizes taken from the prototype. Stroke is 1.65 globally (globals.css), so callers never set strokeWidth. */
export const iconSize = {
  rail: "size-5.5",
  nav: "size-5.25",
  md: "size-5",
  sm: "size-4",
  xs: "size-3.5",
  hero: "size-8.5",
} as const;

export type IconSize = keyof typeof iconSize;

export function Icon({ icon: Glyph, size = "md", className }: { icon: LucideIcon; size?: IconSize; className?: string }) {
  return <Glyph aria-hidden className={cn("shrink-0", iconSize[size], className)} />;
}
