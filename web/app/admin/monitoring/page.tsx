"use client";

import { useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { AlertTriangle, ArrowRight, CloudOff, Radio, RefreshCw } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { StatusChip } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput, TextareaInput } from "@/components/ui/form";
import { api, ApiError, errorText, type ActiveSession, type ViolationTimeline } from "@/lib/api/client";
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
  const [extendOpen, setExtendOpen] = useState(false);
  const [minutes, setMinutes] = useState("");
  const [reason, setReason] = useState("");
  const [extendErrors, setExtendErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

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

  const openExtend = () => {
    setMinutes("");
    setReason("");
    setExtendErrors({});
    setExtendOpen(true);
  };

  const submitExtend = async () => {
    if (!detail) return;
    const errors: Record<string, string> = {};
    const mins = Number(minutes);
    if (!/^\d+$/.test(minutes.trim()) || mins < 1 || mins > 240) errors.seconds = t.minutesRule;
    if (reason.trim() === "" || reason.trim().length > 500) errors.reason = t.reasonRule;
    setExtendErrors(errors);
    if (Object.keys(errors).length > 0) return;
    setSaving(true);
    try {
      const ext = await api.monitoring.extend(detail.session_id, { seconds: mins * 60, reason: reason.trim() });
      toast.success(t.extendSaved, { description: t.extendSavedDescription(Math.round(ext.total_seconds / 60)) });
      setExtendOpen(false);
    } catch (err) {
      if (err instanceof ApiError && err.fields) setExtendErrors(err.fields);
      toast.danger(t.extendFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  const columns: Column<ActiveSession>[] = [
    {
      key: "student",
      header: t.colStudent,
      render: (s) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={{ name: s.student_name }} size="sm" />
          <div className="min-w-0">
            <p className="font-medium text-foreground">{s.student_name}</p>
            <p className="text-label text-muted">{s.tryout_title}</p>
          </div>
        </div>
      ),
    },
    { key: "subtest", header: t.colActiveSubtest, render: (s) => s.subtest?.name ?? "—" },
    { key: "started", header: t.colStarted, render: (s) => elapsedLabel(s.started_at, t) },
    {
      key: "violations",
      header: t.colViolations,
      render: (s) =>
        s.violation_count > 0 ? (
          <StatusChip tone="danger" dot>
            {s.violation_count}
          </StatusChip>
        ) : (
          <StatusChip tone="neutral" dot>
            0
          </StatusChip>
        ),
    },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (s) => (
        <Button variant="ghost" size="sm" className="text-accent" onPress={() => openDetail(s)}>
          {t.viewDetail}
          <Icon icon={ArrowRight} size="xs" />
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
          <span className="flex items-center gap-2 text-label text-muted">
            <Icon icon={RefreshCw} size="xs" /> {t.updatedAt(formatDateTime(lastSync.toISOString()))}
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
        <p className="py-14 text-center text-caption text-muted">{t.loading}</p>
      ) : (
        <>
          <div className="mb-5 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-control border border-border bg-surface px-4 py-3 text-caption text-foreground">
            <span className="relative inline-flex size-2.5 shrink-0">
              <span className="absolute inline-flex size-full animate-ping rounded-full motion-reduce:animate-none bg-accent opacity-60" />
              <span className="relative inline-flex size-2.5 rounded-full bg-accent" />
            </span>
            {t.liveSessionsBanner(total)}
            {loadFailed && <span className="text-label text-warning">{t.refreshFailed}</span>}
          </div>

          {sessions.length === 0 ? (
            <EmptyState icon={Radio} title={t.emptyTitle} description={t.emptyDescription} />
          ) : (
            <DataTable
              columns={columns}
              rows={sessions}
              rowKey={(s) => s.session_id}
              footer={
                <>
                  <span>{lastSync ? t.updatedAt(formatDateTime(lastSync.toISOString())) : ""}</span>
                  {total > sessions.length && <span>{t.showingOf(sessions.length, total)}</span>}
                </>
              }
            />
          )}
        </>
      )}

      <FormModal
        isOpen={!!detail && !extendOpen}
        onOpenChange={(open) => !open && setDetail(null)}
        title={t.detailTitle}
        description={detail ? t.detailDescription(detail.student_name, detail.tryout_title) : undefined}
        footer={
          <>
            <Button variant="secondary" isDisabled={!detail?.subtest} onPress={openExtend}>
              {t.extendButton}
            </Button>
            <Button variant="ghost" onPress={() => setDetail(null)}>
              {t.close}
            </Button>
          </>
        }
      >
        {detail && (
          <>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-4 rounded-tile border border-border p-5 text-small">
              <div>
                <dt className="mb-1 text-label text-muted">{t.startedAt}</dt>
                <dd className="font-medium text-foreground">{formatDateTime(detail.started_at)}</dd>
              </div>
              <div>
                <dt className="mb-1 text-label text-muted">{t.activeSubtest}</dt>
                <dd className="font-medium text-foreground">{detail.subtest?.name ?? "—"}</dd>
              </div>
            </dl>
            <div>
              {timelineFailed ? (
                <p className="text-caption text-danger">{t.timelineError}</p>
              ) : timeline === null ? (
                <p className="text-caption text-muted">{t.loading}</p>
              ) : (
                <>
                  <h3 className="mb-3 font-display text-title">{t.violationHistoryTitle(timeline.total)}</h3>
                  {timeline.violations.length === 0 ? (
                    <p className="text-caption text-muted">{t.noViolations}</p>
                  ) : (
                    <ul>
                      {timeline.violations.map((v) => (
                        <li key={v.id} className="relative flex items-baseline gap-3 py-3 not-last:after:absolute not-last:after:top-7 not-last:after:-bottom-3 not-last:after:left-1 not-last:after:w-px not-last:after:bg-border">
                          <span className="z-10 size-2 shrink-0 rounded-full bg-danger" aria-hidden />
                          <div className="flex min-w-0 flex-1 flex-wrap items-baseline justify-between gap-x-3">
                            <span className="flex items-center gap-2 text-small font-medium text-foreground">
                              <Icon icon={AlertTriangle} size="xs" className="text-danger" />
                              {v.type === "screenshot_attempt" ? t.violationScreenshot : t.violationFullscreenExit}
                            </span>
                            <time className="text-label text-muted">{formatDateTime(v.occurred_at)}</time>
                          </div>
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

      <FormModal
        isOpen={extendOpen}
        onOpenChange={(open) => !open && setExtendOpen(false)}
        title={t.extendTitle}
        description={detail ? t.extendDescription(detail.student_name, detail.subtest?.name ?? t.extendNoSubtest) : undefined}
        footer={
          <>
            <Button variant="ghost" onPress={() => setExtendOpen(false)} isDisabled={saving}>
              {t.cancel}
            </Button>
            <Button variant="primary" onPress={() => void submitExtend()} isDisabled={saving}>
              {t.save}
            </Button>
          </>
        }
      >
        <TextInput
          label={t.minutesLabel}
          required
          inputMode="numeric"
          value={minutes}
          onChange={(e) => setMinutes(e.target.value)}
          error={extendErrors.seconds}
        />
        <TextareaInput
          label={t.reasonLabel}
          required
          rows={3}
          maxLength={500}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          error={extendErrors.reason}
        />
      </FormModal>
    </AppShell>
  );
}
