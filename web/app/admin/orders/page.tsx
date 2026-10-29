"use client";

import { useEffect, useState } from "react";
import { Button, Chip, toast } from "@heroui/react";
import { CheckCircle2, ChevronRight, ClipboardList, CloudOff, FileImage, Undo2, Wallet, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { OrderStatusBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { api, ApiError, errorText, type Order } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useOrdersT } from "@/lib/i18n/dict/orders";

type Tab = Order["status"] | "all";

const PAGE_SIZE = 50;

export default function OrdersPage() {
  const t = useOrdersT();
  const tabs: { key: Tab; label: string }[] = [
    { key: "all", label: t.tabAll },
    { key: "pending", label: t.tabPending },
    { key: "confirmed", label: t.tabConfirmed },
    { key: "cancelled", label: t.tabCancelled },
  ];
  const [rows, setRows] = useState<Order[] | null>(null);
  const [total, setTotal] = useState(0);
  const [tab, setTab] = useState<Tab>("all");
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [detail, setDetail] = useState<Order | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [resolving, setResolving] = useState<"credit" | "refund" | null>(null);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.adminOrders
      .list({ status: tab === "all" ? undefined : tab, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.orders);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [tab, reloadKey]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const switchTab = (next: Tab) => {
    setRows(null);
    setLoadFailed(false);
    setTab(next);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const page = await api.adminOrders.list({
        status: tab === "all" ? undefined : tab,
        limit: PAGE_SIZE,
        offset: rows?.length ?? 0,
      });
      setRows((prev) => {
        const seen = new Set((prev ?? []).map((order) => order.id));
        return [...(prev ?? []), ...page.orders.filter((order) => !seen.has(order.id))];
      });
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const closeDetail = () => {
    setDetail(null);
    setCancelling(false);
    setResolving(null);
    setReason("");
    setReasonError("");
  };

  const settle = (updated: Order) => {
    setRows((prev) =>
      (prev ?? []).flatMap((o) => (o.id !== updated.id ? [o] : tab === "all" || tab === updated.status ? [updated] : [])),
    );
    toast.success(updated.status === "confirmed" ? t.orderConfirmed : t.orderCancelled, {
      description: updated.status === "confirmed" ? t.confirmNotifDescription : undefined,
    });
    closeDetail();
  };

  const confirm = async (order: Order) => {
    setBusy(true);
    try {
      settle(await api.adminOrders.confirm(order.id));
    } catch (err) {
      toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setBusy(false);
    }
  };

  const cancel = async (order: Order) => {
    setBusy(true);
    setReasonError("");
    try {
      settle(await api.adminOrders.cancel(order.id, reason));
    } catch (err) {
      setReasonError(err instanceof ApiError ? (err.fields?.reason ?? "") : "");
      toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setBusy(false);
    }
  };

  const resolve = async (order: Order, action: "credit" | "refund") => {
    if (!order.payment_exception) return;
    setBusy(true);
    setReasonError("");
    try {
      const call = action === "credit" ? api.paymentExceptions.credit : api.paymentExceptions.refund;
      await call(order.payment_exception.id, reason);
      toast.success(action === "credit" ? t.exceptionCredited : t.exceptionRefunded);
      closeDetail();
      setReloadKey((key) => key + 1);
    } catch (err) {
      setReasonError(err instanceof ApiError ? (err.fields?.reason ?? "") : "");
      toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setBusy(false);
    }
  };

  const openProof = async (assetId: string) => {
    try {
      const link = await api.assets.url(assetId);
      window.open(link.url, "_blank", "noopener,noreferrer");
    } catch (err) {
      toast.danger(t.proofOpenFailed, { description: errorText(err, t.loadErrorDescription) });
    }
  };

  const methodShort = (o: Order) =>
    o.payment_method === "wallet" ? t.paymentWalletShort : o.payment_method === "gateway" ? t.paymentGatewayShort : t.paymentManualShort;
  const methodLong = (o: Order) =>
    o.payment_method === "wallet" ? t.paymentWalletLong : o.payment_method === "gateway" ? t.paymentGatewayLong : t.paymentManualLong;

  const columns: Column<Order>[] = [
    { key: "code", header: t.colCode, render: (o) => <span className="font-mono text-xs text-muted">{o.code}</span> },
    { key: "student", header: t.colStudent, render: (o) => o.student_name },
    { key: "product", header: t.colProduct, render: (o) => o.product_title },
    { key: "amount", header: t.colTotal, render: (o) => formatIDR(o.price) },
    { key: "payment", header: t.colMethod, render: (o) => <span className="text-muted">{methodShort(o)}</span> },
    { key: "date", header: t.colDate, render: (o) => new Date(o.created_at).toLocaleString() },
    {
      key: "status",
      header: t.colStatus,
      render: (o) => (
        <span className="inline-flex flex-wrap items-center gap-1.5">
          <OrderStatusBadge status={o.status} />
          {o.payment_exception && (
            <Chip size="sm" className="bg-warning-soft font-medium text-warning-soft-foreground">
              {t.needsReview}
            </Chip>
          )}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: () => <ChevronRight className="ml-auto size-4 text-muted" />,
    },
  ];

  return (
    <AppShell title={t.pageTitle} description={rows === null ? undefined : t.orderCount(total)}>
      <div className="mb-4 flex flex-wrap gap-2">
        {tabs.map((item) => (
          <button
            key={item.key}
            onClick={() => switchTab(item.key)}
            className={cn(
              "rounded-full border px-3.5 py-1.5 text-sm font-medium transition-colors",
              tab === item.key ? "border-accent bg-accent text-accent-foreground" : "border-border text-muted hover:border-accent/40 hover:text-foreground",
            )}
          >
            {item.label}
          </button>
        ))}
      </div>

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
      ) : rows === null ? (
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      ) : rows.length === 0 ? (
        <EmptyState icon={ClipboardList} title={t.emptyTitle} />
      ) : (
        <>
          <DataTable columns={columns} rows={rows} rowKey={(o) => o.id} onRowClick={setDetail} />
          {rows.length < total && (
            <div className="mt-4 flex justify-center">
              <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                {loadingMore ? t.loadingMore : t.loadMore}
              </Button>
            </div>
          )}
        </>
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && closeDetail()}
        title={detail?.code ?? ""}
        description={detail?.product_title}
        footer={
          detail?.payment_exception ? (
            resolving ? (
              <>
                <Button variant="ghost" isDisabled={busy} onPress={() => setResolving(null)}>
                  {t.back}
                </Button>
                <Button variant="primary" isDisabled={busy || reason.trim() === ""} onPress={() => resolve(detail, resolving)}>
                  {resolving === "credit" ? t.creditConfirm : t.refundConfirm}
                </Button>
              </>
            ) : (
              <>
                <Button variant="ghost" isDisabled={busy} onPress={closeDetail}>
                  {t.close}
                </Button>
                <Button variant="secondary" isDisabled={busy} onPress={() => setResolving("refund")}>
                  <Undo2 className="size-3.5" /> {t.refundAction}
                </Button>
                <Button variant="primary" isDisabled={busy} onPress={() => setResolving("credit")}>
                  <Wallet className="size-3.5" /> {t.creditAction}
                </Button>
              </>
            )
          ) : detail?.status === "pending" ? (
            cancelling ? (
              <>
                <Button variant="ghost" isDisabled={busy} onPress={() => setCancelling(false)}>
                  {t.back}
                </Button>
                <Button variant="danger" isDisabled={busy || reason.trim() === ""} onPress={() => cancel(detail)}>
                  <XCircle className="size-3.5" /> {t.cancelConfirm}
                </Button>
              </>
            ) : (
              <>
                <Button variant="danger" isDisabled={busy} onPress={() => setCancelling(true)}>
                  <XCircle className="size-3.5" /> {t.cancel}
                </Button>
                {detail.payment_method !== "gateway" && (
                  <Button variant="primary" isDisabled={busy} onPress={() => confirm(detail)}>
                    <CheckCircle2 className="size-3.5" /> {t.confirmPayment}
                  </Button>
                )}
              </>
            )
          ) : (
            <Button variant="ghost" onPress={closeDetail}>
              {t.close}
            </Button>
          )
        }
      >
        {detail && (
          <>
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <p className="text-xs text-muted">{t.detailStudent}</p>
                <p className="font-medium text-foreground">{detail.student_name}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailTotal}</p>
                <p className="font-medium text-foreground">{formatIDR(detail.price)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailMethod}</p>
                <p className="font-medium text-foreground">{methodLong(detail)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailStatus}</p>
                <OrderStatusBadge status={detail.status} />
              </div>
              {detail.status === "cancelled" && (
                <div className="col-span-2">
                  <p className="text-xs text-muted">{t.detailCancelReason}</p>
                  <p className="font-medium text-foreground">{detail.cancel_reason}</p>
                </div>
              )}
            </div>
            {detail.proof_asset_id ? (
              <button
                type="button"
                onClick={() => detail.proof_asset_id && openProof(detail.proof_asset_id)}
                className="flex items-center gap-3 rounded-lg border border-dashed border-border p-4 text-left transition-colors hover:border-accent/50"
              >
                <FileImage className="size-8 shrink-0 text-muted" />
                <div>
                  <p className="text-sm font-medium text-foreground">{t.proofOpen}</p>
                  <p className="text-xs text-muted">{t.proofUploadedBy}</p>
                </div>
              </button>
            ) : (
              detail.payment_method === "manual" && <p className="text-xs text-muted">{t.proofNotUploaded}</p>
            )}
            {detail.payment_exception && (
              <div className="rounded-lg border border-warning/40 bg-warning-soft p-3 text-sm text-warning-soft-foreground">
                <p className="font-semibold">{t.needsReview}</p>
                <p className="mt-1 text-xs">{t.exceptionKind[detail.payment_exception.kind]}</p>
                <p className="mt-1 text-xs">{t.exceptionPaid(formatIDR(detail.payment_exception.paid_amount))}</p>
              </div>
            )}
            {resolving && (
              <TextareaInput
                label={resolving === "credit" ? t.creditReasonLabel : t.refundReasonLabel}
                description={resolving === "credit" ? t.creditReasonHint : t.refundReasonHint}
                value={reason}
                error={reasonError}
                rows={3}
                maxLength={500}
                onChange={(e) => setReason(e.target.value)}
              />
            )}
            {cancelling && (
              <TextareaInput
                label={t.cancelReasonLabel}
                description={t.cancelReasonHint}
                value={reason}
                error={reasonError}
                rows={3}
                maxLength={500}
                onChange={(e) => setReason(e.target.value)}
              />
            )}
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
