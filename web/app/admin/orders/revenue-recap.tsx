"use client";

import { useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { CircleDollarSign, ReceiptText, Wallet } from "lucide-react";
import { DataTable, type Column } from "@/components/data-table";
import { Panel, SectionHeader } from "@/components/page-blocks";
import { StatCard } from "@/components/stat-card";
import { TextInput } from "@/components/ui/form";
import { api, type RevenueRecap } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useReportT } from "@/lib/i18n/dict/report";

const DAYS_PER_PAGE = 31;

type Range = { from: string; to: string };
type Bucket = RevenueRecap["by_payment_method"][number];
type Day = RevenueRecap["days"][number];

/** Admin-only. Empty range means the server's default window (the last 30 Asia/Jakarta days). */
export function RevenueRecapSection() {
  const t = useReportT();
  const [draft, setDraft] = useState<Range>({ from: "", to: "" });
  const [range, setRange] = useState<Range>({ from: "", to: "" });
  const [recap, setRecap] = useState<RevenueRecap | null>(null);
  const [days, setDays] = useState<Day[]>([]);
  const [failed, setFailed] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.reports
      .revenue({ from: range.from || undefined, to: range.to || undefined, limit: DAYS_PER_PAGE })
      .then((next) => {
        if (cancelled) return;
        setRecap(next);
        setDays(next.days);
        setDraft((prev) => (prev.from === "" && prev.to === "" ? { from: next.from, to: next.to } : prev));
      })
      .catch((err: { status?: number }) => {
        if (!cancelled) setFailed(err.status === 422 ? t.rangeInvalid : t.loadFailed);
      });
    return () => {
      cancelled = true;
    };
  }, [range, reloadKey, t.rangeInvalid, t.loadFailed]);

  const apply = () => {
    setRecap(null);
    setFailed("");
    setRange(draft);
    setReloadKey((key) => key + 1);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const next = await api.reports.revenue({
        from: range.from || undefined,
        to: range.to || undefined,
        limit: DAYS_PER_PAGE,
        offset: days.length,
      });
      setDays((prev) => [...prev, ...next.days]);
    } catch {
      setFailed(t.loadFailed);
    } finally {
      setLoadingMore(false);
    }
  };

  const bucketColumns = (labels: Record<string, string>): Column<Bucket>[] => [
    { key: "key", header: t.colGroup, render: (b) => labels[b.key] ?? b.key },
    { key: "count", header: t.colCount, render: (b) => <span className="tabular-nums">{b.count}</span> },
    { key: "total", header: t.colTotal, render: (b) => <span className="font-medium tabular-nums">{formatIDR(b.total)}</span> },
  ];
  const dayColumns: Column<Day>[] = [
    { key: "date", header: t.colDate, render: (d) => d.date },
    { key: "orders", header: t.colOrders, render: (d) => <span className="tabular-nums">{d.orders} · {formatIDR(d.orders_total)}</span> },
    { key: "topups", header: t.colTopups, render: (d) => <span className="tabular-nums">{d.topups} · {formatIDR(d.topups_total)}</span> },
  ];

  return (
    <Panel as="section" tone="plain" radius="tile" className="mb-6 border border-border">
      <SectionHeader title={t.title} description={t.description} />
      <form
        className="mb-5 flex flex-wrap items-end gap-3"
        onSubmit={(event) => {
          event.preventDefault();
          apply();
        }}
      >
        <TextInput label={t.from} type="date" value={draft.from} onChange={(e) => setDraft((r) => ({ ...r, from: e.target.value }))} />
        <TextInput label={t.to} type="date" value={draft.to} onChange={(e) => setDraft((r) => ({ ...r, to: e.target.value }))} />
        <Button variant="secondary" size="sm" type="submit" isDisabled={recap === null && failed === ""}>
          {recap === null && failed === "" ? t.applying : t.apply}
        </Button>
      </form>

      {failed !== "" ? (
        <div className="flex items-center gap-3 text-caption text-danger">
          {failed}
          <Button variant="ghost" size="sm" onPress={apply}>
            {t.retry}
          </Button>
        </div>
      ) : recap === null ? (
        <p className="py-6 text-center text-caption text-muted">{t.loading}</p>
      ) : (
        <div className="space-y-5">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatCard compact icon={ReceiptText} label={t.statOrders} value={String(recap.orders.count)} />
            <StatCard compact icon={CircleDollarSign} label={t.statOrdersTotal} value={formatIDR(recap.orders.total)} />
            <StatCard compact icon={Wallet} label={t.statTopups} value={String(recap.topups_approved.count)} />
            <StatCard compact icon={CircleDollarSign} label={t.statTopupsTotal} value={formatIDR(recap.topups_approved.total)} />
          </div>
          <p className="text-label text-muted">{t.topupsNote}</p>
          {recap.orders.count === 0 && recap.topups_approved.count === 0 ? (
            <p className="text-caption text-muted">{t.noActivity}</p>
          ) : (
            <>
              <div className="grid gap-4 lg:grid-cols-2">
                <div>
                  <h3 className="mb-2 font-display text-title">{t.byMethod}</h3>
                  <DataTable columns={bucketColumns(t.method)} rows={recap.by_payment_method} rowKey={(b) => b.key} />
                </div>
                <div>
                  <h3 className="mb-2 font-display text-title">{t.byProduct}</h3>
                  <DataTable columns={bucketColumns(t.product)} rows={recap.by_product_type} rowKey={(b) => b.key} />
                </div>
              </div>
              <DataTable columns={dayColumns} rows={days} rowKey={(d) => d.date} />
              {days.length < recap.days_total && (
                <div className="flex justify-center">
                  <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                    {loadingMore ? t.loadingMore : t.loadMoreDays}
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      )}
    </Panel>
  );
}
