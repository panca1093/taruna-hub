"use client";

import { useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { Banknote, CheckCircle2, ChevronRight, CloudOff, FileImage, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { TopupStatusBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { api, ApiError, errorText, type Topup } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useTopupsT } from "@/lib/i18n/dict/topups";

type Tab = Topup["status"] | "all";

const PAGE_SIZE = 50;

export default function TopupsPage() {
  const t = useTopupsT();
  const tabs: { key: Tab; label: string }[] = [
    { key: "all", label: t.tabAll },
    { key: "pending", label: t.tabPending },
    { key: "approved", label: t.tabApproved },
    { key: "rejected", label: t.tabRejected },
  ];
  const [rows, setRows] = useState<Topup[] | null>(null);
  const [total, setTotal] = useState(0);
  const [tab, setTab] = useState<Tab>("all");
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [detail, setDetail] = useState<Topup | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.adminTopups
      .list({ status: tab === "all" ? undefined : tab, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.topups);
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
      const page = await api.adminTopups.list({
        status: tab === "all" ? undefined : tab,
        limit: PAGE_SIZE,
        offset: rows?.length ?? 0,
      });
      setRows((prev) => {
        const seen = new Set((prev ?? []).map((topup) => topup.id));
        return [...(prev ?? []), ...page.topups.filter((topup) => !seen.has(topup.id))];
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
    setRejecting(false);
    setReason("");
    setReasonError("");
  };

  const settle = (updated: Topup) => {
    setRows((prev) =>
      (prev ?? []).flatMap((x) => (x.id !== updated.id ? [x] : tab === "all" || tab === updated.status ? [updated] : [])),
    );
    toast.success(updated.status === "approved" ? t.topupApproved : t.topupRejected, {
      description: updated.status === "approved" ? t.approveNotifDescription : undefined,
    });
    closeDetail();
  };

  const approve = async (topup: Topup) => {
    setBusy(true);
    try {
      settle(await api.adminTopups.approve(topup.id));
    } catch (err) {
      toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setBusy(false);
    }
  };

  const reject = async (topup: Topup) => {
    setBusy(true);
    setReasonError("");
    try {
      settle(await api.adminTopups.reject(topup.id, reason));
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

  const columns: Column<Topup>[] = [
    { key: "code", header: t.colCode, render: (x) => <span className="font-mono text-xs text-muted">{x.code}</span> },
    { key: "student", header: t.colStudent, render: (x) => x.student_name },
    { key: "amount", header: t.colAmount, render: (x) => formatIDR(x.amount) },
    { key: "date", header: t.colDate, render: (x) => new Date(x.created_at).toLocaleString() },
    { key: "status", header: t.colStatus, render: (x) => <TopupStatusBadge status={x.status} /> },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: () => <ChevronRight className="ml-auto size-4 text-muted" />,
    },
  ];

  return (
    <AppShell title={t.pageTitle} description={rows === null ? undefined : t.topupCount(total)}>
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
        <EmptyState icon={Banknote} title={t.emptyTitle} />
      ) : (
        <>
          <DataTable columns={columns} rows={rows} rowKey={(x) => x.id} onRowClick={setDetail} />
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
        description={detail ? formatIDR(detail.amount) : undefined}
        footer={
          detail?.status === "pending" ? (
            rejecting ? (
              <>
                <Button variant="ghost" isDisabled={busy} onPress={() => setRejecting(false)}>
                  {t.back}
                </Button>
                <Button variant="danger" isDisabled={busy || reason.trim() === ""} onPress={() => reject(detail)}>
                  <XCircle className="size-3.5" /> {t.rejectConfirm}
                </Button>
              </>
            ) : (
              <>
                <Button variant="danger" isDisabled={busy} onPress={() => setRejecting(true)}>
                  <XCircle className="size-3.5" /> {t.reject}
                </Button>
                {detail.method !== "gateway" && (
                  <Button variant="primary" isDisabled={busy} onPress={() => approve(detail)}>
                    <CheckCircle2 className="size-3.5" /> {t.approve}
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
                <p className="text-xs text-muted">{t.detailAmount}</p>
                <p className="font-medium text-foreground">{formatIDR(detail.amount)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailStatus}</p>
                <TopupStatusBadge status={detail.status} />
              </div>
              {detail.status === "rejected" && (
                <div className="col-span-2">
                  <p className="text-xs text-muted">{t.detailRejectReason}</p>
                  <p className="font-medium text-foreground">{detail.reject_reason}</p>
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
              <p className="text-xs text-muted">{t.proofNotUploaded}</p>
            )}
            {rejecting && (
              <TextareaInput
                label={t.rejectReasonLabel}
                description={t.rejectReasonHint}
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
