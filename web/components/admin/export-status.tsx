"use client";

import { useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { api, errorText, type Export } from "@/lib/api/client";
import { formatDateTime } from "@/lib/mock-data";
import { useExportsT } from "@/lib/i18n/dict/exports";
import { StatusChip } from "@/components/status-badge";

const POLL_MS = 3000;
const LATEST = 5;

const inProgress = (e: Export) => e.status === "queued" || e.status === "running";

const tone = {
  queued: "warning",
  running: "warning",
  ready: "success",
  failed: "danger",
} as const;

export function ExportStatus({ kind, refreshKey }: { kind: Export["kind"]; refreshKey?: number }) {
  const t = useExportsT();
  const [rows, setRows] = useState<Export[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [opening, setOpening] = useState<string | null>(null);

  const [tick, setTick] = useState(0);
  const polling = rows.some(inProgress);

  useEffect(() => {
    let live = true;
    api.exports
      .list({ kind, limit: LATEST })
      .then((list) => {
        if (!live) return;
        setRows(list.exports);
        setError(null);
      })
      .catch((err) => {
        if (live) setError(errorText(err, t.loadFailed));
      });
    return () => {
      live = false;
    };
  }, [kind, refreshKey, tick, t.loadFailed]);

  useEffect(() => {
    if (!polling) return;
    const timer = setInterval(() => setTick((n) => n + 1), POLL_MS);
    return () => clearInterval(timer);
  }, [polling]);

  async function download(row: Export) {
    setOpening(row.id);
    try {
      const link = await api.exports.downloadLink(row.id);
      window.location.assign(link.url);
      setError(null);
    } catch (err) {
      setError(errorText(err, t.downloadFailed));
    } finally {
      setOpening(null);
    }
  }

  if (rows.length === 0 && !error) return null;

  const label = {
    queued: t.statusQueued,
    running: t.statusRunning,
    ready: t.statusReady,
    failed: t.statusFailed,
  } as const;

  return (
    <section aria-label={t.title} className="flex flex-col gap-2">
      <h3 className="text-label font-medium text-muted">{t.title}</h3>
      {error && (
        <p role="alert" className="text-danger">
          {error}
        </p>
      )}
      <ul className="flex flex-col divide-y divide-separator rounded-lg border border-separator">
        {rows.map((row) => (
          <li key={row.id} className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
            <div className="flex min-w-0 flex-col">
              <span className="truncate font-medium">{row.file_name ?? t.pendingFile}</span>
              <span className="text-label text-muted">
                {formatDateTime(row.created_at)}
                {row.status === "failed" && row.error_code ? ` · ${t.failedMessages[row.error_code] ?? t.failedReason(row.error_code)}` : ""}
              </span>
            </div>
            <div className="flex items-center gap-3">
              <StatusChip tone={tone[row.status]} dot>
                {label[row.status]}
              </StatusChip>
              {row.status === "ready" && (
                <Button size="sm" variant="secondary" isPending={opening === row.id} onPress={() => void download(row)}>
                  {opening === row.id ? t.downloading : t.download}
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
