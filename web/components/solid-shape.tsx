import type { ReactNode } from "react";
import type { ShapeKind } from "@/lib/rich-doc";
import { cn } from "@/lib/cn";

const hidden = { strokeDasharray: "4 4", opacity: 0.55 };

const SHAPES: Record<ShapeKind, ReactNode> = {
  kubus: (
    <>
      <rect x="18" y="35" width="50" height="50" />
      <path d="M18 35l16-16h50v50l-16 16M68 35l16-16" />
      <path d="M34 19v50h50M34 69L18 85" style={hidden} />
    </>
  ),
  balok: (
    <>
      <rect x="8" y="45" width="62" height="34" />
      <path d="M8 45l20-16h62v34l-20 16M70 45l20-16" />
      <path d="M28 29v34h62M28 63L8 79" style={hidden} />
    </>
  ),
  tabung: (
    <>
      <ellipse cx="50" cy="24" rx="28" ry="9" />
      <path d="M22 24v54M78 24v54M22 78a28 9 0 0 0 56 0" />
      <path d="M22 78a28 9 0 0 1 56 0" style={hidden} />
    </>
  ),
  kerucut: (
    <>
      <path d="M50 12L20 78M50 12l30 66M20 78a30 9 0 0 0 60 0" />
      <path d="M20 78a30 9 0 0 1 60 0M50 12v66" style={hidden} />
    </>
  ),
  bola: (
    <>
      <circle cx="50" cy="50" r="36" />
      <path d="M14 50a36 11 0 0 0 72 0" />
      <path d="M14 50a36 11 0 0 1 72 0" style={hidden} />
    </>
  ),
  limas: (
    <>
      <path d="M50 14L14 80h52l20-18zM50 14l16 66" />
      <path d="M14 80l20-18h52M34 62L50 14" style={hidden} />
    </>
  ),
  prisma: (
    <>
      <path d="M14 80h42L35 46zM35 46l30-18 21 34-30 18M56 80l30-18" />
      <path d="M14 80l30-18h42M44 62l21-34" style={hidden} />
    </>
  ),
  persegi: <rect x="20" y="20" width="60" height="60" />,
  persegi_panjang: <rect x="10" y="29" width="80" height="42" />,
  segitiga: <path d="M50 16L12 82h76z" />,
  lingkaran: <circle cx="50" cy="50" r="36" />,
  trapesium: <path d="M32 24h36l20 54H12z" />,
  jajar_genjang: <path d="M30 26h58L70 74H12z" />,
  belah_ketupat: <path d="M50 12l36 38-36 38-36-38z" />,
  layang_layang: <path d="M50 10l30 32-30 48-30-48z" />,
};

/** Line drawing of a solid or flat shape in the current text colour; hidden edges are dashed. */
export function SolidShape({ kind, className, label }: { kind: ShapeKind; className?: string; label?: string }) {
  return (
    <svg
      viewBox="0 0 100 100"
      {...(label ? { role: "img", "aria-label": label } : { "aria-hidden": true })}
      className={cn("fill-none stroke-current", className)}
      strokeWidth={2}
      strokeLinejoin="round"
      strokeLinecap="round"
    >
      {SHAPES[kind]}
    </svg>
  );
}
