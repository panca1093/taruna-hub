"use client";

import { useEffect, useState } from "react";
import { Button, Chip } from "@heroui/react";
import { AlertTriangle, CloudOff, Radio, RefreshCw } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { api, type ActiveSession, type ViolationTimeline } from "@/lib/api/client";
import { useMonitoringT } from "@/lib/i18n/dict/monitoring";

const POLL_MS = 30_000;
const PAGE_SIZE = 200;

const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(
    new Date(iso),
  );

function elapsedLabel(startedAt: string, t: ReturnType<typeof useMonitoringT>) {
  const mins = Math.max(0, Math.round((Date.now() - new Date(startedAt).getTime()) / 60000));
  return t.minutesAgo(mins);
}

export default function MonitoringPage() {
  const t = useMonitoringT();
  const [sessions, setSessions] = useState<ActiveSession[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [lastSync, setLastSync] = useState<Date | null>(null);
  const [detail, setDetail] = useState<ActiveSession | null>(null);
  const [timeline, setTimeline] = useState<ViolationTimeline | null>(null);
  const [timelineFailed, setTimelineFailed] = useState(false);

  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const poll = () =>
      api.monitoring
        .active({ limit: PAGE_SIZE })
        .then((page) => {
          if (cancelled) return;
          setSessions(page.sessions);
          setTotal(page.total);
          setLastSync(new Date());
          setLoadFailed(false);
        })
        .catch(() => {
          // A failed poll keeps the last good rows on screen; only a first load with nothing to show is an error.
          if (!cancelled) setLoadFailed(true);
        });
    void poll();
    const interval = setInterval(() => void poll(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [reloadKey]);

  useEffect(() => {
    if (!detail) return;
    let cancelled = false;
    api.monitoring
      .violations(detail.session_id)
      .then((loaded) => {
        if (!cancelled) setTimeline(loaded);
      })
      .catch(() => {
        if (!cancelled) setTimelineFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [detail]);

  const openDetail = (s: ActiveSession) => {
    setTimeline(null);
    setTimelineFailed(false);
    setDetail(s);
  };

  const columns: Column<ActiveSession>[] = [
    {
      key: "student",
      header: t.colStudent,
      render: (s) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={{ name: s.student_name }} size="sm" />
          <span className="font-medium text-foreground">{s.student_name}</span>
        </div>
      ),
    },
    { key: "tryout", header: t.colTryout, render: (s) => s.tryout_title },
    { key: "subtest", header: t.colActiveSubtest, render: (s) => s.subtest?.name ?? "—" },
    { key: "started", header: t.colStarted, render: (s) => elapsedLabel(s.started_at, t) },
    {
      key: "violations",
      header: t.colViolations,
      render: (s) =>
        s.violation_count > 0 ? (
          <Chip size="sm" className="bg-danger-soft text-danger-soft-foreground gap-1 font-medium">
            <AlertTriangle className="size-3" /> {s.violation_count}
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
        <Button variant="ghost" size="sm" onPress={() => openDetail(s)}>
          {t.viewDetail}
        </Button>
      ),
    },
  ];

  return (
    <AppShell
      title={t.pageTitle}
      description={t.pageDescription}
      actions={
        lastSync && (
          <span className="flex items-center gap-1.5 text-xs text-muted">
            <RefreshCw className="size-3" /> {t.updatedAt(formatDateTime(lastSync.toISOString()))}
          </span>
        )
      }
    >
      {sessions === null && loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" size="sm" onPress={() => setReloadKey((key) => key + 1)}>
              {t.retry}
            </Button>
          }
        />
      ) : sessions === null ? (
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      ) : (
        <>
          <div className="mb-4 flex items-center gap-2 text-sm text-foreground">
            <span className="relative inline-flex size-2.5">
              <span className="absolute inline-flex size-full animate-ping rounded-full motion-reduce:animate-none bg-accent opacity-60" />
              <span className="relative inline-flex size-2.5 rounded-full bg-accent" />
            </span>
            <Radio className="size-4 text-muted" />
            {t.liveSessionsBanner(total)}
            {loadFailed && <span className="text-xs text-warning">{t.refreshFailed}</span>}
          </div>

          {sessions.length === 0 ? (
            <EmptyState icon={Radio} title={t.emptyTitle} description={t.emptyDescription} />
          ) : (
            <DataTable columns={columns} rows={sessions} rowKey={(s) => s.session_id} />
          )}
        </>
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={t.detailTitle}
        description={detail ? t.detailDescription(detail.student_name, detail.tryout_title) : undefined}
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
                <p className="font-medium text-foreground">{formatDateTime(detail.started_at)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.activeSubtest}</p>
                <p className="font-medium text-foreground">{detail.subtest?.name ?? "—"}</p>
              </div>
            </div>
            <div>
              {timelineFailed ? (
                <p className="text-sm text-danger">{t.timelineError}</p>
              ) : timeline === null ? (
                <p className="text-sm text-muted">{t.loading}</p>
              ) : (
                <>
                  <p className="mb-2 text-[13px] font-medium text-foreground">{t.violationHistoryTitle(timeline.total)}</p>
                  {timeline.violations.length === 0 ? (
                    <p className="text-sm text-muted">{t.noViolations}</p>
                  ) : (
                    <ul className="space-y-2">
                      {timeline.violations.map((v) => (
                        <li key={v.id} className="flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger-soft px-3 py-2 text-sm">
                          <AlertTriangle className="size-3.5 shrink-0 text-danger" />
                          <span className="text-danger-soft-foreground">
                            {v.type === "screenshot_attempt" ? t.violationScreenshot : t.violationFullscreenExit}
                          </span>
                          <span className="ml-auto text-xs text-muted">{formatDateTime(v.occurred_at)}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </>
              )}
            </div>
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
