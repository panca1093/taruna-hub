"use client";

import { useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, ChevronRight, ClipboardList, CloudOff, Undo2, Wallet, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { OrderStatusBadge, StatusChip } from "@/components/status-badge";
import { DetailAmount, DetailGrid, DetailItem, LoadingNote, ProofButton } from "@/components/admin-detail";
import { FilterPills } from "@/components/filter-pills";
import { Icon } from "@/components/icon";
import { FormModal } from "@/components/ui/form-modal";
import { TextareaInput } from "@/components/ui/form";
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
    { key: "code", header: t.colCode, render: (o) => <span className="font-mono text-label text-muted">{o.code}</span> },
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
            <StatusChip tone="warning" dot>
              {t.needsReview}
            </StatusChip>
          )}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: () => <Icon icon={ChevronRight} size="xs" className="ml-auto text-muted" />,
    },
  ];

  return (
    <AppShell title={t.pageTitle} description={t.pageDescription}>
      <FilterPills
        className="mb-5"
        ariaLabel={t.pageTitle}
        options={tabs.map((item) => ({ id: item.key, label: item.label }))}
        value={tab}
        onChange={(id) => switchTab(id as Tab)}
      />

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
        <LoadingNote>{t.loading}</LoadingNote>
      ) : rows.length === 0 ? (
        <EmptyState icon={ClipboardList} title={t.emptyTitle} />
      ) : (
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(o) => o.id}
          onRowClick={setDetail}
          footer={
            <>
              <span>{t.shownOf(rows.length, total)}</span>
              {rows.length < total && (
                <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                  {loadingMore ? t.loadingMore : t.loadMore}
                </Button>
              )}
            </>
          }
        />
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
                  <Icon icon={Undo2} size="sm" /> {t.refundAction}
                </Button>
                <Button variant="primary" isDisabled={busy} onPress={() => setResolving("credit")}>
                  <Icon icon={Wallet} size="sm" /> {t.creditAction}
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
                  <Icon icon={XCircle} size="sm" /> {t.cancelConfirm}
                </Button>
              </>
            ) : (
              <>
                <Button variant="danger" isDisabled={busy} onPress={() => setCancelling(true)}>
                  <Icon icon={XCircle} size="sm" /> {t.cancel}
                </Button>
                {detail.payment_method !== "gateway" && (
                  <Button variant="primary" isDisabled={busy} onPress={() => confirm(detail)}>
                    <Icon icon={CheckCircle2} size="sm" /> {t.confirmPayment}
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
            <DetailAmount>{formatIDR(detail.price)}</DetailAmount>
            <DetailGrid>
              <DetailItem label={t.detailStudent}>{detail.student_name}</DetailItem>
              <DetailItem label={t.detailMethod}>{methodLong(detail)}</DetailItem>
              <DetailItem label={t.detailStatus}>
                <OrderStatusBadge status={detail.status} />
              </DetailItem>
              {detail.status === "cancelled" && (
                <DetailItem label={t.detailCancelReason} wide>
                  {detail.cancel_reason}
                </DetailItem>
              )}
            </DetailGrid>
            {detail.proof_asset_id ? (
              <ProofButton title={t.proofOpen} hint={t.proofUploadedBy} onPress={() => detail.proof_asset_id && openProof(detail.proof_asset_id)} />
            ) : (
              detail.payment_method === "manual" && <p className="text-label text-muted">{t.proofNotUploaded}</p>
            )}
            {detail.payment_exception && (
              <div className="rounded-tile border border-warning/40 bg-warning-soft p-4 text-small text-warning-soft-foreground">
                <p className="font-semibold">{t.needsReview}</p>
                <p className="mt-1 text-caption">{t.exceptionKind[detail.payment_exception.kind]}</p>
                <p className="mt-1 text-caption">{t.exceptionPaid(formatIDR(detail.payment_exception.paid_amount))}</p>
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
