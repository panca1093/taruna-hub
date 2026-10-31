"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, toast } from "@heroui/react";
import { CheckCircle2, Clock, CloudOff, RefreshCw, ShieldAlert, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { OrderStatusBadge } from "@/components/status-badge";
import { LinkButton } from "@/components/ui/link-button";
import { cn } from "@/lib/cn";
import { api, ApiError, errorText, type Order } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useOrderDetailT } from "@/lib/i18n/dict/order-detail";

type LoadState = "loading" | "ready" | "missing" | "failed";

const POLL_MS = 5000;

export default function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const t = useOrderDetailT();
  const [order, setOrder] = useState<Order | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [refreshing, setRefreshing] = useState(false);
  const [refreshFailed, setRefreshFailed] = useState(false);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  // Initial load; refresh() below is the one reader for poll, focus and the manual button.
  useEffect(() => {
    let cancelled = false;
    api.orders
      .get(id)
      .then((fresh) => {
        if (cancelled) return;
        setOrder(fresh);
        setState("ready");
      })
      .catch((err) => {
        if (!cancelled) setState(err instanceof ApiError && err.status === 404 ? "missing" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [id, reloadKey]);

  const view: LoadState = state === "ready" && order?.id !== id ? "loading" : state;

  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      const fresh = await api.orders.get(id);
      setOrder((prev) => (prev === null || prev.id === fresh.id ? fresh : prev));
      setRefreshFailed(false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) setState("missing");
      else setRefreshFailed(true);
    } finally {
      setRefreshing(false);
    }
  }, [id]);

  const polling = order?.payment_method === "gateway" && order.status === "pending";

  useEffect(() => {
    if (!polling) return;
    const timer = setInterval(() => void refresh(), POLL_MS);
    const onVisible = () => {
      if (document.visibilityState === "visible") void refresh();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [polling, refresh]);

  const cancel = async () => {
    setCancelling(true);
    try {
      setOrder(await api.orders.cancel(id));
      setConfirmingCancel(false);
    } catch (err) {
      toast.danger(t.cancelFailed, { description: errorText(err, t.loadErrorDescription) });
      setConfirmingCancel(false);
      void refresh();
    } finally {
      setCancelling(false);
    }
  };

  if (view !== "ready" || order === null) {
    return (
      <AppShell title={t.title} maxWidth="max-w-5xl">
        {view === "loading" ? (
          <div role="status" aria-label={t.loading} className="h-72 animate-pulse rounded-xl border border-border bg-surface-secondary" />
        ) : (
          <EmptyState
            icon={CloudOff}
            title={view === "missing" ? t.notFoundTitle : t.loadErrorTitle}
            description={view === "missing" ? t.notFoundDescription : t.loadErrorDescription}
            action={
              view === "missing" ? (
                <LinkButton href="/student/orders" variant="primary" size="sm">
                  {t.backToOrders}
                </LinkButton>
              ) : (
                <Button
                  variant="primary"
                  size="sm"
                  onPress={() => {
                    setState("loading");
                    setReloadKey((key) => key + 1);
                  }}
                >
                  {t.retry}
                </Button>
              )
            }
          />
        )}
      </AppShell>
    );
  }

  const kecermatan = order.product_type === "kecermatan";
  const pendingGateway = order.status === "pending" && order.payment_method === "gateway";
  const banner = bannerOf(order, t);
  const held = order.payment_exception;

  return (
    <AppShell title={t.title} description={order.code} maxWidth="max-w-5xl">
      <nav className="mb-4 text-xs text-muted">
        <Link href="/student/orders" className="hover:text-foreground hover:underline">
          {t.crumbOrders}
        </Link>
      </nav>

      <div className={cn("flex items-start gap-4 rounded-2xl p-5 sm:p-6", banner.className)} role="status" aria-live="polite">
        <span className="inline-flex size-11 shrink-0 items-center justify-center rounded-full bg-surface text-foreground">
          <banner.icon className="size-5" />
        </span>
        <div className="min-w-0">
          <h2 className="font-display text-xl font-semibold text-foreground">{banner.title}</h2>
          <p className="mt-1 text-sm text-muted">{banner.body}</p>
          {order.status === "cancelled" && order.cancel_reason !== "" && (
            <p className="mt-1 text-sm text-muted">{t.cancelReason(order.cancel_reason)}</p>
          )}
        </div>
      </div>

      {held && (
        <div className="mt-4 flex items-start gap-3 rounded-xl border border-border bg-warning-soft p-4 text-warning-soft-foreground" role="status">
          <ShieldAlert className="mt-0.5 size-5 shrink-0" />
          <div className="text-sm">
            <p className="font-semibold">{t.heldTitle}</p>
            <p className="mt-1">{t.held[held.kind](formatIDR(held.paid_amount))}</p>
            <p className="mt-1">{t.heldContact}</p>
          </div>
        </div>
      )}

      {refreshFailed && <p className="mt-4 text-sm text-danger">{t.refreshFailed}</p>}

      <div className="mt-6 grid gap-6 lg:grid-cols-[1.4fr_1fr]">
        <Card className="h-fit gap-4 p-5 sm:p-6">
          <div>
            <p className="text-xs font-medium text-muted uppercase">{t.summaryProduct}</p>
            <p className="font-display mt-1 text-lg font-semibold text-foreground">{order.product_title}</p>
            {kecermatan && order.included_attempts !== null && (
              <p className="mt-1 text-sm text-muted">{t.attempts(order.included_attempts)}</p>
            )}
          </div>
          {order.items.length > 0 && (
            <div className="border-t border-border pt-4">
              <p className="text-xs font-medium text-muted uppercase">{t.itemsTitle(order.items.length)}</p>
              <ul className="mt-2 space-y-1 text-sm text-foreground">
                {order.items.map((item) => (
                  <li key={item.tryout_id}>{item.title}</li>
                ))}
              </ul>
            </div>
          )}

          {order.status === "confirmed" && (
            <LinkButton
              href={kecermatan && order.kecermatan_test_id ? `/student/kecermatan/${order.kecermatan_test_id}` : "/student/tryouts"}
              variant="primary"
              size="lg"
              className="w-full sm:w-fit"
            >
              {kecermatan ? t.openKecermatan : t.openTryouts}
            </LinkButton>
          )}
          {order.status === "cancelled" && !held && (
            <LinkButton href="/catalog" variant="secondary" size="md" className="w-full sm:w-fit">
              {t.backToCatalog}
            </LinkButton>
          )}

          {pendingGateway && (
            <div className="flex flex-wrap items-center gap-2">
              {order.redirect_url && (
                <LinkButton href={order.redirect_url} variant="primary" size="lg">
                  {t.continuePayment}
                </LinkButton>
              )}
              {confirmingCancel ? (
                <div className="flex w-full flex-col gap-2 rounded-lg border border-border bg-surface-secondary p-3 text-sm" role="alertdialog" aria-label={t.cancelConfirmTitle}>
                  <p className="font-medium text-foreground">{t.cancelConfirmTitle}</p>
                  <p className="text-muted">{t.cancelConfirmBody}</p>
                  <div className="flex gap-2">
                    <Button size="sm" variant="danger" onPress={cancel} isDisabled={cancelling}>
                      {cancelling ? t.cancelling : t.cancelConfirmYes}
                    </Button>
                    <Button size="sm" variant="ghost" onPress={() => setConfirmingCancel(false)} isDisabled={cancelling}>
                      {t.cancelConfirmNo}
                    </Button>
                  </div>
                </div>
              ) : (
                <Button variant="ghost" size="lg" onPress={() => setConfirmingCancel(true)}>
                  {t.cancelOrder}
                </Button>
              )}
            </div>
          )}
        </Card>

        <Card className="h-fit gap-3 p-5 sm:p-6">
          <Row label={t.summaryCode} value={<span className="font-mono">{order.code}</span>} />
          <Row label={t.summaryAmount} value={formatIDR(order.price)} />
          <Row label={t.summaryMethod} value={t.methodLabel[order.payment_method]} />
          <Row label={t.summaryStatus} value={<OrderStatusBadge status={order.status} />} />
          <Row label={t.summaryCreated} value={new Date(order.created_at).toLocaleString()} />
          {order.confirmed_at && <Row label={t.summaryConfirmed} value={new Date(order.confirmed_at).toLocaleString()} />}
          {order.cancelled_at && <Row label={t.summaryCancelled} value={new Date(order.cancelled_at).toLocaleString()} />}
          {order.proof_asset_id && <p className="text-xs text-muted">{t.proofSent}</p>}
          {order.status === "pending" && (
            <Button variant="secondary" size="sm" onPress={() => void refresh()} isDisabled={refreshing}>
              <RefreshCw className={cn("size-3.5", refreshing && "animate-spin motion-reduce:animate-none")} />
              {refreshing ? t.refreshing : t.refresh}
            </Button>
          )}
        </Card>
      </div>
    </AppShell>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 text-sm">
      <span className="text-muted">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}

function bannerOf(order: Order, t: ReturnType<typeof useOrderDetailT>) {
  if (order.status === "confirmed") {
    return {
      icon: CheckCircle2,
      className: "bg-success-soft",
      title: t.confirmedTitle,
      body:
        order.product_type === "kecermatan" && order.included_attempts !== null
          ? t.confirmedBodyKecermatan(order.included_attempts)
          : t.confirmedBodyTryout,
    };
  }
  if (order.status === "cancelled") {
    return { icon: XCircle, className: "bg-danger-soft", title: t.cancelledTitle, body: t.cancelledBody };
  }
  if (order.payment_method === "gateway") {
    return {
      icon: Clock,
      className: "bg-accent-soft",
      title: t.pendingGatewayTitle,
      body: order.redirect_url ? t.pendingGatewayBody : t.pendingGatewayNoLink,
    };
  }
  return { icon: Clock, className: "bg-accent-soft", title: t.pendingManualTitle, body: t.pendingManualBody };
}
