"use client";

import { useMemo, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, ChevronRight, ClipboardList, FileImage, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { OrderStatusBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { cn } from "@/lib/cn";
import { findUser, formatDateTime, formatIDR, getOrderTitle, orders as initialOrders } from "@/lib/mock-data";
import { useOrdersT } from "@/lib/i18n/dict/orders";
import type { Order, OrderStatus } from "@/lib/types";

export default function OrdersPage() {
  const t = useOrdersT();
  const tabs: { key: OrderStatus | "all"; label: string }[] = [
    { key: "all", label: t.tabAll },
    { key: "pending", label: t.tabPending },
    { key: "confirmed", label: t.tabConfirmed },
    { key: "cancelled", label: t.tabCancelled },
  ];
  const [rows, setRows] = useState<Order[]>(initialOrders);
  const [tab, setTab] = useState<OrderStatus | "all">("all");
  const [detail, setDetail] = useState<Order | null>(null);

  const filtered = useMemo(() => (tab === "all" ? rows : rows.filter((o) => o.status === tab)), [rows, tab]);

  const updateStatus = (id: string, status: OrderStatus) => {
    setRows((prev) =>
      prev.map((o) => (o.id === id ? { ...o, status, confirmedAt: status === "confirmed" ? new Date().toISOString() : o.confirmedAt } : o)),
    );
    toast.success(status === "confirmed" ? t.orderConfirmed : t.orderCancelled, {
      description: status === "confirmed" ? t.confirmNotifDescription : undefined,
    });
    setDetail(null);
  };

  const columns: Column<Order>[] = [
    { key: "code", header: t.colCode, render: (o) => <span className="font-mono text-xs text-muted">{o.code}</span> },
    { key: "student", header: t.colStudent, render: (o) => findUser(o.userId)?.name ?? o.userId },
    { key: "product", header: t.colProduct, render: (o) => getOrderTitle(o) },
    { key: "amount", header: t.colTotal, render: (o) => formatIDR(o.amount) },
    {
      key: "payment",
      header: t.colMethod,
      render: (o) => (
        <span className="text-muted">{o.paymentMode === "gateway" ? t.paymentGatewayShort : o.paymentMode === "wallet" ? t.paymentWalletShort : t.paymentManualShort}</span>
      ),
    },
    { key: "date", header: t.colDate, render: (o) => formatDateTime(o.createdAt) },
    { key: "status", header: t.colStatus, render: (o) => <OrderStatusBadge status={o.status} /> },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: () => <ChevronRight className="ml-auto size-4 text-muted" />,
    },
  ];

  return (
    <AppShell title={t.pageTitle} description={t.orderCount(rows.length)}>
      <div className="mb-4 flex flex-wrap gap-2">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={cn(
              "rounded-full border px-3.5 py-1.5 text-sm font-medium transition-colors",
              tab === t.key ? "border-accent bg-accent text-accent-foreground" : "border-border text-muted hover:border-accent/40 hover:text-foreground",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={ClipboardList} title={t.emptyTitle} />
      ) : (
        <DataTable columns={columns} rows={filtered} rowKey={(o) => o.id} onRowClick={setDetail} />
      )}

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detail?.code ?? ""}
        description={detail ? getOrderTitle(detail) : undefined}
        footer={
          detail?.status === "pending" ? (
            <>
              <Button variant="danger" onPress={() => detail && updateStatus(detail.id, "cancelled")}>
                <XCircle className="size-3.5" /> {t.cancel}
              </Button>
              <Button variant="primary" onPress={() => detail && updateStatus(detail.id, "confirmed")}>
                <CheckCircle2 className="size-3.5" /> {t.confirmPayment}
              </Button>
            </>
          ) : (
            <Button variant="ghost" onPress={() => setDetail(null)}>
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
                <p className="font-medium text-foreground">{findUser(detail.userId)?.name}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailTotal}</p>
                <p className="font-medium text-foreground">{formatIDR(detail.amount)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailMethod}</p>
                <p className="font-medium text-foreground">
                  {detail.paymentMode === "gateway" ? t.paymentGatewayLong : detail.paymentMode === "wallet" ? t.paymentWalletLong : t.paymentManualLong}
                </p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailStatus}</p>
                <OrderStatusBadge status={detail.status} />
              </div>
            </div>
            {detail.paymentProofUrl ? (
              <div className="flex items-center gap-3 rounded-lg border border-dashed border-border p-4">
                <FileImage className="size-8 shrink-0 text-muted" />
                <div>
                  <p className="text-sm font-medium text-foreground">bukti-transfer.jpg</p>
                  <p className="text-xs text-muted">{t.proofUploadedBy}</p>
                </div>
              </div>
            ) : (
              <p className="text-xs text-muted">{t.proofNotUploaded}</p>
            )}
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
