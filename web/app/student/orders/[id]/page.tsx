"use client";

import { use, useCallback, useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { ArrowRight, Check, Clock, CloudOff, RefreshCw, ShieldAlert, X } from "lucide-react";
import { Icon } from "@/components/icon";
import { NoteBox, Panel } from "@/components/page-blocks";
import { Breadcrumb } from "@/components/page-heading";
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

  const polling = state === "ready" && order?.payment_method === "gateway" && order.status === "pending";

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
          <div role="status" aria-label={t.loading} className="h-72 animate-pulse rounded-card border border-border bg-surface-secondary" />
        ) : (
          <EmptyState
            icon={CloudOff}
            title={view === "missing" ? t.notFoundTitle : t.loadErrorTitle}
            description={view === "missing" ? t.notFoundDescription : t.loadErrorDescription}
            action={
              view === "missing" ? (
                <LinkButton href="/student/orders" variant="primary">
                  {t.backToOrders}
                </LinkButton>
              ) : (
                <Button
                  variant="primary"
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
    <AppShell maxWidth="max-w-5xl">
      <Breadcrumb
        items={[
          { label: t.crumbDashboard, href: "/student" },
          { label: t.crumbOrders, href: "/student/orders" },
          { label: order.code },
        ]}
      />

      <div className={cn("flex items-center gap-4 rounded-menu p-5 sm:p-6", banner.className)} role="status" aria-live="polite">
        <span className="inline-flex size-12 shrink-0 items-center justify-center rounded-full bg-surface text-foreground">
          <Icon icon={banner.icon} size="md" />
        </span>
        <div className="min-w-0">
          <h1 className="font-display text-hero text-balance">{banner.title}</h1>
          <p className="mt-2 text-caption text-muted">{banner.body}</p>
          {order.status === "cancelled" && order.cancel_reason !== "" && (
            <p className="mt-1 text-caption text-muted">{t.cancelReason(order.cancel_reason)}</p>
          )}
        </div>
      </div>

      {held && (
        <div className="mt-4 flex items-start gap-3 rounded-tile bg-warning-soft p-4 text-warning-soft-foreground" role="status">
          <Icon icon={ShieldAlert} size="md" className="mt-0.5" />
          <div className="text-caption">
            <p className="font-semibold">{t.heldTitle}</p>
            <p className="mt-1">{t.held[held.kind](formatIDR(held.paid_amount))}</p>
            <p className="mt-1">{t.heldContact}</p>
          </div>
        </div>
      )}

      {refreshFailed && <p className="mt-4 text-small text-danger">{t.refreshFailed}</p>}

      <div className="mt-6 grid gap-7 lg:grid-cols-[minmax(0,1.5fr)_minmax(17.5rem,0.65fr)] lg:items-start">
        <Panel className="space-y-5">
          <div>
            <p className="text-label text-muted">{t.summaryProduct}</p>
            <h2 className="mt-2 font-display text-heading">{order.product_title}</h2>
            {kecermatan && order.included_attempts !== null && (
              <p className="mt-1 text-caption text-muted">{t.attempts(order.included_attempts)}</p>
            )}
          </div>
          {order.items.length > 0 && (
            <div className="border-t border-border pt-5">
              <p className="text-label text-muted">{t.itemsTitle(order.items.length)}</p>
              <ul className="mt-2 space-y-1.5 text-small text-foreground">
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
              className="w-full sm:w-fit"
            >
              {kecermatan ? t.openKecermatan : t.openTryouts}
              <Icon icon={ArrowRight} size="sm" />
            </LinkButton>
          )}
          {order.status === "cancelled" && !held && (
            <LinkButton href="/catalog" variant="secondary" className="w-full sm:w-fit">
              {t.backToCatalog}
              <Icon icon={ArrowRight} size="sm" />
            </LinkButton>
          )}

          {pendingGateway && (
            <div className="flex flex-wrap items-center gap-2.5">
              {order.redirect_url && (
                <LinkButton href={order.redirect_url} variant="primary">
                  {t.continuePayment}
                  <Icon icon={ArrowRight} size="sm" />
                </LinkButton>
              )}
              {confirmingCancel ? (
                <div className="flex w-full flex-col gap-2 rounded-tile border border-border bg-background p-4 text-caption" role="alertdialog" aria-label={t.cancelConfirmTitle}>
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
                <Button variant="ghost" onPress={() => setConfirmingCancel(true)}>
                  {t.cancelOrder}
                </Button>
              )}
            </div>
          )}
        </Panel>

        <Panel as="aside">
          <p className="font-mono text-label text-muted">{order.code}</p>
          <p className="mt-3 font-display text-heading">{formatIDR(order.price)}</p>
          <dl className="mt-3 text-caption">
            <Row label={t.summaryMethod} value={t.methodLabel[order.payment_method]} />
            <Row label={t.summaryStatus} value={<OrderStatusBadge status={order.status} />} />
            <Row label={t.summaryCreated} value={new Date(order.created_at).toLocaleString()} />
            {order.confirmed_at && <Row label={t.summaryConfirmed} value={new Date(order.confirmed_at).toLocaleString()} />}
            {order.cancelled_at && <Row label={t.summaryCancelled} value={new Date(order.cancelled_at).toLocaleString()} />}
          </dl>
          {order.proof_asset_id && <NoteBox className="mt-3">{t.proofSent}</NoteBox>}
          {order.status === "pending" && (
            <Button variant="secondary" fullWidth className="mt-4" onPress={() => void refresh()} isDisabled={refreshing}>
              <Icon icon={RefreshCw} size="xs" className={cn(refreshing && "animate-spin motion-reduce:animate-none")} />
              {refreshing ? t.refreshing : t.refresh}
            </Button>
          )}
        </Panel>
      </div>
    </AppShell>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-5 py-3">
      <dt className="text-muted">{label}</dt>
      <dd className="text-right font-semibold text-foreground">{value}</dd>
    </div>
  );
}

function bannerOf(order: Order, t: ReturnType<typeof useOrderDetailT>) {
  if (order.status === "confirmed") {
    return {
      icon: Check,
      className: "bg-success-soft text-success-soft-foreground",
      title: t.confirmedTitle,
      body:
        order.product_type === "kecermatan" && order.included_attempts !== null
          ? t.confirmedBodyKecermatan(order.included_attempts)
          : t.confirmedBodyTryout,
    };
  }
  if (order.status === "cancelled") {
    return { icon: X, className: "bg-peach", title: t.cancelledTitle, body: t.cancelledBody };
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
