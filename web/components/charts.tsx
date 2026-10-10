"use client";

import { motion } from "framer-motion";
import { cn } from "@/lib/cn";

interface BarDatum {
  label: string;
  value: number;
}

export function BarChart({ data, className, valueFormatter }: { data: BarDatum[]; className?: string; valueFormatter?: (v: number) => string }) {
  const max = Math.max(...data.map((d) => d.value), 1);

  return (
    <div className={cn("flex h-40 items-stretch gap-2.5", className)}>
      {data.map((d, i) => (
        <div key={d.label} className="group flex flex-1 flex-col items-center gap-2">
          <div className="relative flex w-full flex-1 items-end">
            <motion.div
              initial={{ height: 0 }}
              animate={{ height: d.value === 0 ? "0%" : `${Math.max(4, (d.value / max) * 100)}%` }}
              transition={{ duration: 0.5, delay: i * 0.04, ease: [0.32, 0.72, 0, 1] }}
              className="w-full rounded-t-md bg-gradient-ring opacity-90 transition-opacity group-hover:opacity-100"
            />
            <span className="pointer-events-none absolute -top-6 left-1/2 -translate-x-1/2 rounded-md bg-surface-tertiary px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap text-foreground shadow-surface">
              {valueFormatter ? valueFormatter(d.value) : d.value}
            </span>
          </div>
          <span className="font-mono text-[10px] text-muted">{d.label}</span>
        </div>
      ))}
    </div>
  );
}

export function Sparkline({ data, className, width = 160, height = 40 }: { data: number[]; className?: string; width?: number; height?: number }) {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const step = width / (data.length - 1);
  const points = data.map((v, i) => `${i * step},${height - ((v - min) / range) * height}`).join(" ");

  return (
    <svg width={width} height={height} className={className} viewBox={`0 0 ${width} ${height}`}>
      <defs>
        <linearGradient id="sparkline-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--ring-start)" stopOpacity="0.25" />
          <stop offset="100%" stopColor="var(--ring-start)" stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon points={`0,${height} ${points} ${width},${height}`} fill="url(#sparkline-fill)" />
      <polyline points={points} fill="none" stroke="var(--ring-start)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function DonutStat({
  segments,
  size = 140,
  strokeWidth = 16,
}: {
  segments: { label: string; value: number; className: string }[];
  size?: number;
  strokeWidth?: number;
}) {
  const total = segments.reduce((sum, s) => sum + s.value, 0) || 1;
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;

  const arcs = segments.reduce<{ label: string; className: string; dash: number; offset: number }[]>((acc, s) => {
    const prevOffset = acc.length ? acc[acc.length - 1].offset + acc[acc.length - 1].dash : 0;
    const dash = (s.value / total) * circumference;
    return [...acc, { label: s.label, className: s.className, dash, offset: prevOffset }];
  }, []);

  return (
    <div className="flex flex-wrap items-center gap-5">
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={radius} fill="none" strokeWidth={strokeWidth} className="stroke-border" />
        {arcs.map((a) => (
          <circle
            key={a.label}
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            strokeWidth={strokeWidth}
            strokeDasharray={`${a.dash} ${circumference - a.dash}`}
            strokeDashoffset={-a.offset}
            strokeLinecap="butt"
            className={a.className}
          />
        ))}
      </svg>
      <ul className="space-y-1.5">
        {segments.map((s) => (
          <li key={s.label} className="flex items-center gap-2 text-xs">
            <span className={cn("size-2 rounded-full", s.className.replace("stroke-", "bg-"))} />
            <span className="text-muted">{s.label}</span>
            <span className="font-mono font-medium text-foreground">{s.value}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
