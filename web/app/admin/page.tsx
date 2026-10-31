"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Activity, BarChart3, CalendarDays, Gauge, Receipt, Wallet } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { Metric, MetricStrip } from "@/components/metric-strip";
import { Panel, TextLink } from "@/components/page-blocks";
import { StatusChip } from "@/components/status-badge";
import { api, type ActiveSessionList, type OrderList, type TopupList } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useLocale } from "@/lib/i18n/locale-context";
import { useAdminDashboardT } from "@/lib/i18n/dict/admin-dashboard";

const POLL_MS = 30_000;
const SESSION_ROWS = 5;
const ORDER_ROWS = 3;
const TOPUP_ROWS = 2;

type Block<T> = { data: T | null; failed: boolean; syncedAt: Date | null };
const idle = { data: null, failed: false, syncedAt: null };

function QueueRow({ icon, title, lines, href, action }: { icon: LucideIcon; title: string; lines: string[]; href: string; action: string }) {
  return (
    <li className="flex items-start gap-3 border-t border-border py-4 first:border-t-0 first:pt-0">
      <span className="grid size-9 shrink-0 place-items-center rounded-control bg-warning-soft text-warning-soft-foreground">
        <Icon icon={icon} size="sm" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-small font-medium text-foreground">{title}</p>
        {lines.map((line) => (
          <p key={line} className="mt-0.5 truncate text-label text-muted">
            {line}
          </p>
        ))}
        <TextLink href={href} size="sm" className="mt-1">
          {action}
        </TextLink>
      </div>
    </li>
  );
}

function Shortcut({ icon, title, body, href, action }: { icon: LucideIcon; title: string; body: string; href: string; action: string }) {
  return (
    <Panel radius="tile" padding="sm" className="flex items-center gap-4">
      <span className="hidden size-10.5 shrink-0 place-items-center rounded-control bg-accent-soft text-accent sm:grid">
        <Icon icon={icon} size="md" />
      </span>
      <div className="min-w-0 flex-1">
        <h3 className="font-display text-title">{title}</h3>
        <p className="mt-1 text-label text-muted">{body}</p>
      </div>
      <Link href={href} className="inline-flex min-h-10 shrink-0 items-center rounded-control border border-border bg-surface px-3.5 text-label font-medium outline-none hover:bg-surface-secondary focus-visible:ring-2 focus-visible:ring-focus">
        {action}
      </Link>
    </Panel>
  );
}

export default function AdminDashboardPage() {
  const t = useAdminDashboardT();
  const { locale } = useLocale();
  const [sessions, setSessions] = useState<Block<ActiveSessionList>>(idle);
  const [orders, setOrders] = useState<Block<OrderList>>(idle);
  const [topups, setTopups] = useState<Block<TopupList>>(idle);
  const polling = useRef(false);

  useEffect(() => {
    let cancelled = false;
    // A failed poll keeps the last good data and only flips `failed`, so nothing flashes or zeroes out.
    const track = <T,>(request: Promise<T>, set: React.Dispatch<React.SetStateAction<Block<T>>>) =>
      request.then(
        (data) => {
          if (!cancelled) set({ data, failed: false, syncedAt: new Date() });
        },
        () => {
          if (!cancelled) set((prev) => ({ ...prev, failed: true }));
        },
      );
    const poll = async () => {
      if (polling.current || document.hidden) return;
      polling.current = true;
      await Promise.all([
        track(api.monitoring.active({ limit: SESSION_ROWS }), setSessions),
        track(api.adminOrders.list({ status: "pending", limit: ORDER_ROWS }), setOrders),
        track(api.adminTopups.list({ status: "pending", limit: TOPUP_ROWS }), setTopups),
      ]);
      polling.current = false;
    };
    void poll();
    const interval = setInterval(() => void poll(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  const intlLocale = locale === "en" ? "en-US" : "id-ID";
  const today = new Intl.DateTimeFormat(intlLocale, { weekday: "long", day: "numeric", month: "long", year: "numeric" }).format(new Date());
  const syncedTime = sessions.syncedAt ? new Intl.DateTimeFormat(intlLocale, { hour: "2-digit", minute: "2-digit" }).format(sessions.syncedAt) : null;

  const note = (block: Block<unknown>, text: string) => (block.data ? (block.failed ? t.syncStale : text) : block.failed ? t.unavailableNote : t.loading);

  const sessionRows = sessions.data?.sessions ?? [];
  const pendingOrders = orders.data?.orders ?? [];
  const pendingTopups = topups.data?.topups ?? [];
  const queueLoading = (!orders.data && !orders.failed) || (!topups.data && !topups.failed);
  const queueUnavailable = !orders.data && !topups.data;
  const queueEmpty = !!orders.data && !!topups.data && pendingOrders.length === 0 && pendingTopups.length === 0;
  const orderRest = orders.data ? orders.data.total - pendingOrders.length : 0;
  const topupRest = topups.data ? topups.data.total - pendingTopups.length : 0;

  return (
    <AppShell
      title={t.title}
      description={t.description}
      actions={
        <span className="hidden items-center gap-2 text-caption text-muted sm:flex">
          <Icon icon={CalendarDays} size="sm" />
          {today}
        </span>
      }
    >
      <MetricStrip className="mb-6">
        <Metric size="lg" icon={Activity} value={sessions.data ? String(sessions.data.total) : t.unavailable} label={t.kpiSessions} note={note(sessions, t.kpiSessionsNote)} />
        <Metric size="lg" icon={Receipt} value={orders.data ? String(orders.data.total) : t.unavailable} label={t.kpiOrders} note={note(orders, t.kpiOrdersNote)} />
        <Metric size="lg" icon={Wallet} value={topups.data ? String(topups.data.total) : t.unavailable} label={t.kpiTopups} note={note(topups, t.kpiTopupsNote)} />
      </MetricStrip>

      <div className="grid gap-6 xl:grid-cols-3">
        <section className="min-w-0 overflow-hidden rounded-card border border-border bg-surface xl:col-span-2">
          <div className="flex items-start justify-between gap-4 px-6 pt-6 pb-4 sm:px-7">
            <div className="min-w-0">
              <h2 className="font-display text-heading-sm">{t.liveTitle}</h2>
              <p className="mt-1 text-label text-muted">{t.liveDescription}</p>
            </div>
            <TextLink href="/admin/monitoring">{t.liveLink}</TextLink>
          </div>
          {!sessions.data ? (
            <p className="border-t border-border px-7 py-10 text-center text-caption text-muted">{sessions.failed ? t.liveError : t.loading}</p>
          ) : sessionRows.length === 0 ? (
            <p className="border-t border-border px-7 py-10 text-center text-caption text-muted">{t.liveEmpty}</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-caption">
                <thead>
                  <tr className="border-y border-border bg-surface-secondary">
                    <th className="px-6 py-3 text-left text-label font-medium whitespace-nowrap text-muted sm:px-7">{t.colStudent}</th>
                    <th className="hidden px-5 py-3 text-left text-label font-medium whitespace-nowrap text-muted sm:table-cell">{t.colSubtest}</th>
                    <th className="px-5 py-3 text-left text-label font-medium whitespace-nowrap text-muted">{t.colViolations}</th>
                    <th className="px-5 py-3" />
                  </tr>
                </thead>
                <tbody>
                  {sessionRows.map((s) => (
                    <tr key={s.session_id} className="border-b border-border last:border-b-0">
                      <td className="px-6 py-4 sm:px-7">
                        <div className="flex items-center gap-2.5">
                          <GradientAvatar user={{ name: s.student_name }} size="sm" />
                          <div className="min-w-0">
                            <p className="truncate font-medium text-foreground">{s.student_name}</p>
                            <p className="truncate text-label text-muted">{s.tryout_title}</p>
                          </div>
                        </div>
                      </td>
                      <td className="hidden px-5 py-4 whitespace-nowrap sm:table-cell">{s.subtest?.name ?? t.subtestNone}</td>
                      <td className="px-5 py-4">
                        <StatusChip tone={s.violation_count > 0 ? "danger" : "neutral"} dot>
                          {s.violation_count}
                        </StatusChip>
                      </td>
                      <td className="px-5 py-4 text-right">
                        <TextLink href="/admin/monitoring">{t.detail}</TextLink>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-border px-6 py-3.5 text-label text-muted sm:px-7">
            <span>{syncedTime ? t.syncedAt(syncedTime) : sessions.failed ? t.unavailableNote : t.loading}</span>
            {staleOrTotal(t, sessions, sessionRows.length)}
          </div>
        </section>

        <Panel tone="plain" className="xl:col-span-1">
          <div className="mb-5">
            <h2 className="font-display text-heading-sm">{t.queueTitle}</h2>
            <p className="mt-1 text-label text-muted">{t.queueDescription}</p>
          </div>
          {queueLoading && !orders.data && !topups.data ? (
            <p className="py-6 text-center text-caption text-muted">{t.loading}</p>
          ) : queueUnavailable ? (
            <p className="py-6 text-center text-caption text-muted">{t.queueError}</p>
          ) : queueEmpty ? (
            <p className="py-6 text-center text-caption text-muted">{t.queueEmpty}</p>
          ) : (
            <ul>
              {orders.failed && <li className="py-3 text-label text-warning">{orders.data ? t.syncStale : t.queueOrdersError}</li>}
              {pendingOrders.map((o) => (
                <QueueRow key={o.id} icon={Receipt} title={o.student_name} lines={[o.product_title, formatIDR(o.price)]} href="/admin/orders" action={t.queueOrderAction} />
              ))}
              {orderRest > 0 && <li className="border-t border-border py-3 text-label text-muted">{t.queueOrderMore(orderRest)}</li>}
              {topups.failed && <li className="border-t border-border py-3 text-label text-warning first:border-t-0">{topups.data ? t.syncStale : t.queueTopupsError}</li>}
              {pendingTopups.map((p) => (
                <QueueRow key={p.id} icon={Wallet} title={t.topupItem(p.student_name)} lines={[formatIDR(p.amount)]} href="/admin/topups" action={t.queueTopupAction} />
              ))}
              {topupRest > 0 && <li className="border-t border-border py-3 text-label text-muted">{t.queueTopupMore(topupRest)}</li>}
            </ul>
          )}
        </Panel>
      </div>

      <div className="mt-6 grid gap-6 md:grid-cols-2">
        <Shortcut icon={Gauge} title={t.shortcutKecermatanTitle} body={t.shortcutKecermatanBody} href="/admin/kecermatan" action={t.shortcutKecermatanAction} />
        <Shortcut icon={BarChart3} title={t.shortcutResultsTitle} body={t.shortcutResultsBody} href="/admin/results" action={t.shortcutResultsAction} />
      </div>
    </AppShell>
  );
}

function staleOrTotal(t: ReturnType<typeof useAdminDashboardT>, block: Block<ActiveSessionList>, shown: number) {
  if (block.failed) return <span className="text-warning">{t.syncStale}</span>;
  if (block.data && block.data.total > shown) return <span>{t.liveMore(shown, block.data.total)}</span>;
  return null;
}
