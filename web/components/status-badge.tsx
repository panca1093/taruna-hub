"use client";

import { Chip } from "@heroui/react";
import type {
  Difficulty,
  OrderStatus,
  ProgramCode,
  QuestionType,
  SessionStatus,
} from "@/lib/types";
import { cn } from "@/lib/cn";
import { useStatusBadgeT } from "@/lib/i18n/dict/status-badge";

const orderStatusClassName: Record<OrderStatus, string> = {
  pending: "bg-warning-soft text-warning-soft-foreground",
  confirmed: "bg-success-soft text-success-soft-foreground",
  cancelled: "bg-danger-soft text-danger-soft-foreground",
};

export function OrderStatusBadge({ status }: { status: OrderStatus }) {
  const t = useStatusBadgeT();
  return (
    <Chip size="sm" className={cn("font-medium", orderStatusClassName[status])}>
      {t.orderStatus[status]}
    </Chip>
  );
}

const difficultyClassName: Record<Difficulty, string> = {
  easy: "bg-success-soft text-success-soft-foreground",
  medium: "bg-warning-soft text-warning-soft-foreground",
  hard: "bg-danger-soft text-danger-soft-foreground",
};

export function DifficultyBadge({ difficulty }: { difficulty: Difficulty }) {
  const t = useStatusBadgeT();
  return (
    <Chip size="sm" className={cn("font-medium", difficultyClassName[difficulty])}>
      {t.difficulty[difficulty]}
    </Chip>
  );
}

export function QuestionTypeBadge({ type }: { type: QuestionType }) {
  const t = useStatusBadgeT();
  return (
    <Chip size="sm" variant="secondary" className="border border-border font-medium">
      {t.questionType[type]}
    </Chip>
  );
}

const programColorMap: Record<ProgramCode, string> = {
  SD: "bg-sky-500/12 text-sky-700 dark:text-sky-300",
  SMP: "bg-cyan-500/12 text-cyan-700 dark:text-cyan-300",
  SMA: "bg-accent-soft text-accent-soft-foreground",
  CASN: "bg-amber-500/12 text-amber-700 dark:text-amber-300",
  BUMN: "bg-emerald-500/12 text-emerald-700 dark:text-emerald-300",
  AKPOL: "bg-rose-500/12 text-rose-700 dark:text-rose-300",
};

export function ProgramBadge({ code }: { code: string }) {
  const color = programColorMap[code as ProgramCode] ?? "bg-default text-default-foreground";
  return (
    <Chip size="sm" className={cn("font-medium", color)}>
      {code}
    </Chip>
  );
}

const sessionStatusClassName: Record<SessionStatus, string> = {
  in_progress: "bg-accent-soft text-accent-soft-foreground",
  completed: "bg-success-soft text-success-soft-foreground",
};

export function SessionStatusBadge({ status }: { status: SessionStatus }) {
  const t = useStatusBadgeT();
  return (
    <Chip size="sm" className={cn("font-medium", sessionStatusClassName[status])}>
      {t.sessionStatus[status]}
    </Chip>
  );
}

const tryoutStatusClassName = {
  draft: "bg-warning-soft text-warning-soft-foreground",
  published: "bg-success-soft text-success-soft-foreground",
};

export function TryoutStatusBadge({ status }: { status: "draft" | "published" }) {
  const t = useStatusBadgeT();
  return (
    <Chip size="sm" className={cn("font-medium", tryoutStatusClassName[status])}>
      {t.tryoutStatus[status]}
    </Chip>
  );
}
