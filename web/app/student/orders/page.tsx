"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@heroui/react";
import { CloudOff, Receipt } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { OrderStatusBadge } from "@/components/status-badge";
import { api, errorText, type Order } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useStudentOrdersT } from "@/lib/i18n/dict/student-orders";

const PAGE_SIZE = 50;

export default function StudentOrdersPage() {
  const t = useStudentOrdersT();
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState("");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.orders
      .list({ limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setOrders(page.orders);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const retry = () => {
    setOrders(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    setMoreFailed("");
    try {
      const page = await api.orders.list({ limit: PAGE_SIZE, offset: orders?.length ?? 0 });
      setOrders((prev) => {
        const seen = new Set((prev ?? []).map((order) => order.id));
        return [...(prev ?? []), ...page.orders.filter((order) => !seen.has(order.id))];
      });
      setTotal(page.total);
    } catch (err) {
      setMoreFailed(errorText(err, t.loadErrorDescription));
    } finally {
      setLoadingMore(false);
    }
  };

  return (
    <AppShell title={t.title} description={t.description} maxWidth="max-w-6xl">
      {loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" size="sm" onPress={retry}>
              {t.retry}
            </Button>
          }
        />
      ) : orders === null ? (
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      ) : orders.length === 0 ? (
        <EmptyState icon={Receipt} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <>
          <div className="overflow-hidden rounded-xl border border-border bg-surface">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-surface-secondary text-left text-[11px] font-medium tracking-wide text-muted uppercase">
                    <th className="px-4 py-2.5">{t.colCode}</th>
                    <th className="px-4 py-2.5">{t.colProduct}</th>
                    <th className="px-4 py-2.5">{t.colDate}</th>
                    <th className="px-4 py-2.5">{t.colTotal}</th>
                    <th className="px-4 py-2.5">{t.colMethod}</th>
                    <th className="px-4 py-2.5">{t.colStatus}</th>
                  </tr>
                </thead>
                <tbody>
                  {orders.map((o) => (
                    <tr key={o.id} className="border-b border-border last:border-b-0">
                      <td className="px-4 py-3 font-mono text-xs text-muted">{o.code}</td>
                      <td className="px-4 py-3 text-foreground">
                        <Link href={`/catalog/${o.product_id}`} className="hover:underline">
                          {o.product_title}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-muted">{new Date(o.created_at).toLocaleDateString()}</td>
                      <td className="px-4 py-3 font-medium text-foreground">{formatIDR(o.price)}</td>
                      <td className="px-4 py-3 text-muted">{t.paymentModeLabel[o.payment_method]}</td>
                      <td className="px-4 py-3">
                        <OrderStatusBadge status={o.status} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          {moreFailed !== "" && <p className="mt-3 text-center text-sm text-danger">{moreFailed}</p>}
          {orders.length < total && (
            <div className="mt-4 flex justify-center">
              <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                {loadingMore ? t.loadingMore : t.loadMore}
              </Button>
            </div>
          )}
        </>
      )}
    </AppShell>
  );
}
