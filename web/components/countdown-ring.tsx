"use client";

import { useId } from "react";
import { motion } from "framer-motion";
import { Clock } from "lucide-react";
import { cn } from "@/lib/cn";

interface CountdownRingProps {
  /** 0–100 */
  value: number;
  size?: number;
  strokeWidth?: number;
  className?: string;
  children?: React.ReactNode;
  urgent?: boolean;
  trackClassName?: string;
  animate?: boolean;
}

/**
 * The platform's signature element: a gradient radial ring that stands in for
 * both the exam countdown timer and, elsewhere, score/progress. Hand-rolled
 * (not HeroUI's ProgressCircle) so the stroke can carry the brand gradient
 * and shift to a flat danger color under time pressure.
 */
export function CountdownRing({
  value,
  size = 120,
  strokeWidth = 8,
  className,
  children,
  urgent = false,
  trackClassName,
  animate = true,
}: CountdownRingProps) {
  const gradientId = useId();
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const clamped = Math.min(100, Math.max(0, value));
  const offset = circumference - (clamped / 100) * circumference;

  return (
    <div className={cn("relative inline-flex items-center justify-center", className)} style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <defs>
          <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="var(--ring-start)" />
            <stop offset="100%" stopColor="var(--ring-end)" />
          </linearGradient>
        </defs>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          strokeWidth={strokeWidth}
          className={cn("stroke-border", trackClassName)}
        />
        <motion.circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          stroke={urgent ? "var(--danger)" : `url(#${gradientId})`}
          strokeDasharray={circumference}
          initial={animate ? { strokeDashoffset: circumference } : false}
          animate={{ strokeDashoffset: offset }}
          transition={{ duration: animate ? 0.7 : 0, ease: [0.32, 0.72, 0, 1] }}
        />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center">{children}</div>
    </div>
  );
}

/**
 * Prototype exam timer: an accent-soft pill with a clock and a Geist time string.
 * Display only; the caller passes the already-formatted server-derived time.
 */
export function ExamTimerPill({ time, urgent = false, className, ...rest }: { time: string; urgent?: boolean; className?: string } & Omit<React.HTMLAttributes<HTMLDivElement>, "children">) {
  return (
    <div
      {...rest}
      className={cn(
        "inline-flex min-w-28 items-center justify-center gap-2 rounded-tile px-4 py-2.5 font-display text-heading tabular-nums",
        urgent ? "bg-danger-soft text-danger" : "bg-accent-soft text-accent",
        className,
      )}
    >
      <Clock className="size-4.5" aria-hidden />
      {time}
    </div>
  );
}
