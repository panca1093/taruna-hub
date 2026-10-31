"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, toast } from "@heroui/react";
import { CloudOff, Receipt } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { Panel } from "@/components/page-blocks";
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

  // Coming back from Midtrans lands here: poll the pending gateway orders until the server confirms or cancels them.
  useEffect(() => {
    const pending = (orders ?? []).filter((o) => o.payment_method === "gateway" && o.status === "pending");
    if (pending.length === 0) return;
    const timer = setInterval(() => {
      pending.forEach((o) =>
        api.orders
          .get(o.id)
          .then((fresh) => {
            if (fresh.status !== o.status) {
              setOrders((prev) => (prev ?? []).map((row) => (row.id === fresh.id ? fresh : row)));
            }
          })
          .catch(() => {}),
      );
    }, 5000);
    return () => clearInterval(timer);
  }, [orders]);

  const cancelOrder = async (orderId: string) => {
    try {
      const cancelled = await api.orders.cancel(orderId);
      setOrders((prev) => (prev ?? []).map((row) => (row.id === cancelled.id ? cancelled : row)));
    } catch (err) {
      // A payment that already succeeded answers 409; the poll then shows the confirmed order.
      toast.danger(t.cancelFailed, { description: errorText(err, t.loadErrorDescription) });
    }
  };

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
    <AppShell
      title={t.title}
      description={t.description}
      maxWidth="max-w-5xl"
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.title }]}
    >
      {loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" onPress={retry}>
              {t.retry}
            </Button>
          }
        />
      ) : orders === null ? (
        <p className="py-14 text-center text-small text-muted">{t.loading}</p>
      ) : orders.length === 0 ? (
        <EmptyState icon={Receipt} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <>
          <Panel padding="none" className="px-6 py-2 sm:px-7 sm:py-3">
            <ul>
              {orders.map((o) => (
                <li
                  key={o.id}
                  className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 border-b border-border py-4 text-caption last:border-b-0"
                >
                  <div className="min-w-0 flex-1 basis-56">
                    <Link
                      href={`/student/orders/${o.id}`}
                      className="rounded-chip text-small font-medium text-foreground outline-none hover:underline focus-visible:ring-2 focus-visible:ring-focus"
                    >
                      {o.product_title}
                    </Link>
                    <p className="mt-1 text-label text-muted">
                      <span className="font-mono">{o.code}</span> · {new Date(o.created_at).toLocaleDateString()} · {t.paymentModeLabel[o.payment_method]}
                    </p>
                  </div>
                  <span className="inline-flex flex-wrap items-center gap-2">
                    <OrderStatusBadge status={o.status} />
                    {o.payment_exception && (
                      <span className="rounded-chip bg-warning-soft px-2 py-0.5 text-label font-medium text-warning-soft-foreground">
                        {t.heldBadge}
                      </span>
                    )}
                    {o.payment_method === "gateway" && o.status === "pending" && (
                      <Button size="sm" variant="ghost" onPress={() => cancelOrder(o.id)}>
                        {t.cancelOrder}
                      </Button>
                    )}
                  </span>
                  <strong className="min-w-24 text-right font-display text-heading-sm">{formatIDR(o.price)}</strong>
                </li>
              ))}
            </ul>
          </Panel>
          {moreFailed !== "" && <p className="mt-3 text-center text-small text-danger">{moreFailed}</p>}
          {orders.length < total && (
            <div className="mt-4 flex justify-center">
              <Button variant="secondary" onPress={loadMore} isDisabled={loadingMore}>
                {loadingMore ? t.loadingMore : t.loadMore}
              </Button>
            </div>
          )}
        </>
      )}
    </AppShell>
  );
}
