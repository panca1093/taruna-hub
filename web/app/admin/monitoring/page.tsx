"use client";

import { useEffect, useState } from "react";
import { Button, Chip } from "@heroui/react";
import { AlertTriangle, Radio, RefreshCw } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { findUser, formatDateTime, getSubtest, getTryout, liveSessions, violations } from "@/lib/mock-data";
import { useMonitoringT } from "@/lib/i18n/dict/monitoring";
import type { TryoutSession } from "@/lib/types";

function elapsedLabel(startedAt: string, t: ReturnType<typeof useMonitoringT>) {
  const mins = Math.max(0, Math.round((Date.now() - new Date(startedAt).getTime()) / 60000));
  return t.minutesAgo(mins);
}

export default function MonitoringPage() {
  const t = useMonitoringT();
  const [detail, setDetail] = useState<TryoutSession | null>(null);
  const [lastSync, setLastSync] = useState(new Date());
  const [, forceTick] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setLastSync(new Date());
      forceTick((n) => n + 1);
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  const columns: Column<TryoutSession>[] = [
    {
      key: "student",
      header: t.colStudent,
      render: (s) => {
        const u = findUser(s.userId);
        return u ? (
          <div className="flex items-center gap-2.5">
            <GradientAvatar user={u} size="sm" />
            <span className="font-medium text-foreground">{u.name}</span>
          </div>
        ) : (
          s.userId
        );
      },
    },
    { key: "tryout", header: t.colTryout, render: (s) => getTryout(s.tryoutId)?.title },
    { key: "subtest", header: t.colActiveSubtest, render: (s) => getSubtest(s.currentSubtestId ?? "")?.name ?? "—" },
    { key: "started", header: t.colStarted, render: (s) => elapsedLabel(s.startedAt, t) },
    {
      key: "violations",
      header: t.colViolations,
      render: (s) =>
        s.violationCount > 0 ? (
          <Chip size="sm" className="bg-danger-soft text-danger-soft-foreground gap-1 font-medium">
            <AlertTriangle className="size-3" /> {s.violationCount}
          </Chip>
        ) : (
          <span className="text-muted">0</span>
        ),
    },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (s) => (
        <Button variant="ghost" size="sm" onPress={() => setDetail(s)}>
          {t.viewDetail}
        </Button>
      ),
    },
  ];

  const detailViolations = detail ? violations.filter((v) => v.sessionId === detail.id) : [];

  return (
    <AppShell
      title={t.pageTitle}
      description={t.pageDescription}
      actions={
        <span className="flex items-center gap-1.5 text-xs text-muted">
          <RefreshCw className="size-3" /> {t.updatedAt(formatDateTime(lastSync.toISOString()))}
        </span>
      }
    >
      <div className="mb-4 flex items-center gap-2 text-sm text-foreground">
        <span className="relative inline-flex size-2.5">
          <span className="absolute inline-flex size-full animate-ping rounded-full motion-reduce:animate-none bg-accent opacity-60" />
          <span className="relative inline-flex size-2.5 rounded-full bg-accent" />
        </span>
        <Radio className="size-4 text-muted" />
        {t.liveSessionsBanner(liveSessions.length)}
      </div>

      {liveSessions.length === 0 ? (
        <EmptyState icon={Radio} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <DataTable columns={columns} rows={liveSessions} rowKey={(s) => s.id} />
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={t.detailTitle}
        description={detail ? t.detailDescription(findUser(detail.userId)?.name ?? "", getTryout(detail.tryoutId)?.title ?? "") : undefined}
        footer={
          <Button variant="ghost" onPress={() => setDetail(null)}>
            {t.close}
          </Button>
        }
      >
        {detail && (
          <>
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <p className="text-xs text-muted">{t.startedAt}</p>
                <p className="font-medium text-foreground">{formatDateTime(detail.startedAt)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.activeSubtest}</p>
                <p className="font-medium text-foreground">{getSubtest(detail.currentSubtestId ?? "")?.name ?? "—"}</p>
              </div>
            </div>
            <div>
              <p className="mb-2 text-[13px] font-medium text-foreground">{t.violationHistoryTitle(detailViolations.length)}</p>
              {detailViolations.length === 0 ? (
                <p className="text-sm text-muted">{t.noViolations}</p>
              ) : (
                <ul className="space-y-2">
                  {detailViolations.map((v) => (
                    <li key={v.id} className="flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger-soft px-3 py-2 text-sm">
                      <AlertTriangle className="size-3.5 shrink-0 text-danger" />
                      <span className="text-danger-soft-foreground">
                        {v.type === "screenshot_attempt" ? t.violationScreenshot : t.violationFullscreenExit}
                      </span>
                      <span className="ml-auto text-xs text-muted">{formatDateTime(v.timestamp)}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
