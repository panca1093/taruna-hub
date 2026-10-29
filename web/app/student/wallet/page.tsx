"use client";

import { useEffect, useRef, useState } from "react";
import { Button, Chip, toast } from "@heroui/react";
import { ArrowDownLeft, ArrowUpRight, CloudOff, Copy, FileUp, Wallet as WalletIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { TopupStatusBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import {
  api,
  ApiError,
  errorText,
  type BankTransfer,
  type Topup,
  type Wallet,
  type WalletEntry,
} from "@/lib/api/client";
import { formatIDR, formatSignedIDR } from "@/lib/format-money";
import { useStudentWalletT } from "@/lib/i18n/dict/student-wallet";

const PAGE_SIZE = 50;

const when = (iso: string) => new Date(iso).toLocaleString();

const newKey = () => crypto.randomUUID();

export default function StudentWalletPage() {
  const t = useStudentWalletT();
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [entries, setEntries] = useState<WalletEntry[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [topups, setTopups] = useState<Topup[]>([]);
  const [minTopup, setMinTopup] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [bank, setBank] = useState<BankTransfer | null>(null);
  const [bankFailed, setBankFailed] = useState(false);
  const [gatewayOk, setGatewayOk] = useState(false);
  const [method, setMethod] = useState<"manual" | "gateway">("manual");
  const [amount, setAmount] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const keyRef = useRef(newKey());
  const proofRef = useRef<{ file: File; assetId: string } | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.wallet.get(), api.wallet.entries({ limit: PAGE_SIZE }), api.topups.list({ limit: PAGE_SIZE })])
      .then(([head, page, requests]) => {
        if (cancelled) return;
        setWallet(head);
        setEntries(page.entries);
        setTotal(page.total);
        setTopups(requests.topups);
        setMinTopup(requests.min_topup);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  // Coming back from Midtrans: refresh the page once a pending gateway top-up has been settled by the server.
  useEffect(() => {
    const pending = topups.filter((topup) => topup.method === "gateway" && topup.status === "pending");
    if (pending.length === 0) return;
    const timer = setInterval(() => {
      pending.forEach((topup) =>
        api.topups
          .get(topup.id)
          .then((fresh) => {
            if (fresh.status !== topup.status) setReloadKey((key) => key + 1);
          })
          .catch(() => {}),
      );
    }, 5000);
    return () => clearInterval(timer);
  }, [topups]);

  const retry = () => {
    setWallet(null);
    setEntries(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const cancelTopup = async (topupId: string) => {
    try {
      await api.topups.cancel(topupId);
      setReloadKey((key) => key + 1);
    } catch (err) {
      toast.danger(t.cancelTopupFailed, { description: errorText(err, t.loadErrorDescription) });
    }
  };

  const openModal = () => {
    setModalOpen(true);
    api.orders
      .gateway()
      .then(() => setGatewayOk(true))
      .catch(() => setGatewayOk(false));
    if (bank === null) {
      setBankFailed(false);
      api.orders
        .bankTransfer()
        .then(setBank)
        .catch(() => setBankFailed(true));
    }
  };

  const closeModal = () => {
    setModalOpen(false);
    setMethod("manual");
    setAmount("");
    setFile(null);
    setFieldErrors({});
    setSubmitError("");
    proofRef.current = null;
    keyRef.current = newKey();
  };

  const submit = async () => {
    if (method === "manual" && !file) return;
    setSubmitting(true);
    setFieldErrors({});
    setSubmitError("");
    try {
      if (method === "gateway") {
        const created = await api.topups.create({ method: "gateway", amount, idempotency_key: keyRef.current });
        if (created.redirect_url) {
          // Leaving for Midtrans: the balance changes only when the server hears back from the provider.
          window.location.assign(created.redirect_url);
          return;
        }
        closeModal();
        retry();
        return;
      }
      if (!file) return;
      if (proofRef.current?.file !== file) {
        const asset = await api.assets.uploadProof(file);
        proofRef.current = { file, assetId: asset.id };
      }
      await api.topups.create({
        amount,
        idempotency_key: keyRef.current,
        proof_asset_id: proofRef.current.assetId,
      });
      toast.success(t.toastCreated, { description: t.toastCreatedDescription });
      closeModal();
      retry();
    } catch (err) {
      if (err instanceof ApiError && err.fields?.proof_asset_id) proofRef.current = null;
      if (err instanceof ApiError && err.fields) setFieldErrors(err.fields);
      setSubmitError(errorText(err, t.submitFailed));
    } finally {
      setSubmitting(false);
    }
  };

  const topupCodes = new Map(topups.map((topup) => [topup.id, topup.code]));

  const loadMore = async () => {
    setLoadingMore(true);
    setMoreFailed("");
    try {
      const page = await api.wallet.entries({ limit: PAGE_SIZE, offset: entries?.length ?? 0 });
      setEntries((prev) => {
        const seen = new Set((prev ?? []).map((entry) => entry.id));
        return [...(prev ?? []), ...page.entries.filter((entry) => !seen.has(entry.id))];
      });
      setTotal(page.total);
    } catch (err) {
      setMoreFailed(errorText(err, t.loadErrorDescription));
    } finally {
      setLoadingMore(false);
    }
  };

  return (
    <AppShell title={t.title} description={t.description} maxWidth="max-w-4xl">
      <div className="space-y-6">
        <div className="rounded-xl border border-border bg-surface p-5">
          <span className="inline-flex size-9 items-center justify-center rounded-lg bg-accent-soft text-accent-soft-foreground">
            <WalletIcon className="size-[18px]" strokeWidth={2} />
          </span>
          <div className="mt-3">
            <p className="font-display text-3xl font-semibold tracking-tight text-foreground">
              {wallet === null ? "—" : formatIDR(wallet.balance)}
            </p>
            <p className="mt-0.5 text-sm text-muted">{t.balanceLabel}</p>
          </div>
          <p className="mt-1 text-xs text-muted">{t.topUpHint}</p>
          <Button variant="primary" size="sm" className="mt-3" onPress={openModal} isDisabled={wallet === null}>
            {t.topUpButton}
          </Button>
          <div className="mt-3 grid grid-cols-2 gap-3 border-t border-border pt-3 text-sm">
            <div>
              <p className="text-xs text-muted">{t.totalCredits}</p>
              <p className="font-medium text-success">{wallet === null ? "—" : formatIDR(wallet.total_credits)}</p>
            </div>
            <div>
              <p className="text-xs text-muted">{t.totalDebits}</p>
              <p className="font-medium text-foreground">{wallet === null ? "—" : formatIDR(wallet.total_debits)}</p>
            </div>
          </div>
        </div>

        {!loadFailed && entries !== null && (
          <div className="rounded-xl border border-border bg-surface">
            <p className="border-b border-border px-4 py-3 text-sm font-semibold text-foreground">{t.requestsTitle}</p>
            {topups.length === 0 ? (
              <p className="px-4 py-6 text-center text-sm text-muted">{t.requestsEmpty}</p>
            ) : (
              <ul>
                {topups.map((topup) => (
                  <li key={topup.id} className="flex items-center justify-between gap-3 border-b border-border px-4 py-3 text-sm last:border-b-0">
                    <div>
                      <p className="font-mono text-xs text-muted">
                        {topup.code} · {when(topup.created_at)}
                      </p>
                      <p className="font-medium text-foreground">{formatIDR(topup.amount)}</p>
                      {topup.status === "rejected" && (
                        <p className="text-xs text-danger">{t.rejectReason(topup.reject_reason)}</p>
                      )}
                    </div>
                    <span className="inline-flex items-center gap-2">
                      <TopupStatusBadge status={topup.status} />
                      {topup.method === "gateway" && topup.status === "pending" && (
                        <Button size="sm" variant="ghost" onPress={() => cancelTopup(topup.id)}>
                          {t.cancelTopup}
                        </Button>
                      )}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

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
        ) : entries === null ? (
          <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
        ) : entries.length === 0 ? (
          <EmptyState icon={WalletIcon} title={t.emptyTitle} description={t.emptyDescription} />
        ) : (
          <>
            <div className="overflow-hidden rounded-xl border border-border bg-surface">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-surface-secondary text-left text-[11px] font-medium tracking-wide text-muted uppercase">
                      <th className="px-4 py-2.5">{t.colDate}</th>
                      <th className="px-4 py-2.5">{t.colType}</th>
                      <th className="px-4 py-2.5">{t.colDescription}</th>
                      <th className="px-4 py-2.5">{t.colSource}</th>
                      <th className="px-4 py-2.5 text-right">{t.colAmount}</th>
                      <th className="px-4 py-2.5 text-right">{t.colBalanceAfter}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {entries.map((entry) => {
                      const credit = !entry.amount.startsWith("-");
                      return (
                        <tr key={entry.id} className="border-b border-border last:border-b-0">
                          <td className="px-4 py-3 whitespace-nowrap text-muted">{when(entry.created_at)}</td>
                          <td className="px-4 py-3">
                            <Chip
                              size="sm"
                              className={cn(
                                "font-medium",
                                credit ? "bg-success-soft text-success-soft-foreground" : "bg-surface-secondary text-muted",
                              )}
                            >
                              {credit ? <ArrowDownLeft className="size-3" /> : <ArrowUpRight className="size-3" />}
                              {t.typeLabel[entry.kind] ?? entry.kind}
                            </Chip>
                          </td>
                          <td className="px-4 py-3 text-foreground">{entry.reason === "" ? "—" : entry.reason}</td>
                          <td className="px-4 py-3 text-muted">{t.sourceLabel[entry.source_type] ?? entry.source_type}
                            {entry.source_type === "topup" && topupCodes.get(entry.source_id) && (
                              <span className="ml-1 font-mono text-xs">{topupCodes.get(entry.source_id)}</span>
                            )}
                          </td>
                          <td
                            className={cn(
                              "px-4 py-3 text-right font-mono font-medium whitespace-nowrap",
                              credit ? "text-success" : "text-foreground",
                            )}
                          >
                            {formatSignedIDR(entry.amount)}
                          </td>
                          <td className="px-4 py-3 text-right font-mono text-foreground whitespace-nowrap">
                            {formatIDR(entry.balance_after)}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
            {moreFailed !== "" && <p className="text-center text-sm text-danger">{moreFailed}</p>}
            {entries.length < total && (
              <div className="flex justify-center">
                <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                  {loadingMore ? t.loadingMore : t.loadMore}
                </Button>
              </div>
            )}
          </>
        )}
      </div>

      <FormModal
        isOpen={modalOpen}
        onOpenChange={(open) => !open && closeModal()}
        title={t.topUpTitle}
        description={t.topUpDescription}
        footer={
          <>
            <Button variant="ghost" isDisabled={submitting} onPress={closeModal}>
              {t.cancel}
            </Button>
            <Button
              variant="primary"
              isDisabled={submitting || amount === "" || (method === "manual" && (!file || !bank))}
              onPress={submit}
            >
              {submitting ? t.submitting : method === "gateway" ? t.submitGateway : t.submit}
            </Button>
          </>
        }
      >
        {gatewayOk && (
          <div className="grid grid-cols-2 gap-2">
            {(
              [
                { id: "manual", label: t.methodManual },
                { id: "gateway", label: t.methodGateway },
              ] as const
            ).map((m) => (
              <button
                key={m.id}
                type="button"
                onClick={() => {
                  setMethod(m.id);
                  setSubmitError("");
                  keyRef.current = newKey();
                }}
                className={cn(
                  "rounded-xl border px-3 py-2.5 text-xs font-medium transition-colors",
                  method === m.id
                    ? "border-accent bg-accent-soft text-accent-soft-foreground"
                    : "border-border text-muted hover:border-accent/40",
                )}
              >
                {m.label}
              </button>
            ))}
          </div>
        )}
        <TextInput
          label={t.amountLabel}
          description={t.amountHint(formatIDR(minTopup === "" ? "0" : minTopup))}
          inputMode="numeric"
          value={amount}
          error={fieldErrors.amount}
          onChange={(e) => {
            setAmount(e.target.value.replace(/\D/g, ""));
            setSubmitError("");
            keyRef.current = newKey();
          }}
        />
        {method === "gateway" && <p className="text-xs text-muted">{t.gatewayHint}</p>}
        {method === "manual" && (
        <>
        <div className="space-y-2">
          <p className="text-sm font-semibold text-foreground">{t.bankTitle}</p>
          {bank ? (
            <div className="flex items-center justify-between rounded-lg border border-border bg-surface-secondary px-3.5 py-3">
              <div>
                <p className="text-xs text-muted">
                  {bank.bank_name} — a.n. {bank.account_holder}
                </p>
                <p className="font-mono text-sm font-medium text-foreground">{bank.account_number}</p>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                isIconOnly
                onPress={() => navigator.clipboard?.writeText(bank.account_number)}
              >
                <Copy className="size-3.5" />
              </Button>
            </div>
          ) : (
            <p className={cn("text-xs", bankFailed ? "text-danger" : "text-muted")}>
              {bankFailed ? t.bankUnavailable : t.loadingBank}
            </p>
          )}
        </div>
        <div className="space-y-2">
          <p className="text-sm font-semibold text-foreground">{t.proofTitle}</p>
          <input
            ref={fileRef}
            type="file"
            accept="image/png,image/jpeg,image/webp"
            className="hidden"
            onChange={(e) => {
              setFile(e.target.files?.[0] ?? null);
              setSubmitError("");
              keyRef.current = newKey();
            }}
          />
          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            className="flex w-full flex-col items-center gap-2 rounded-xl border border-dashed border-border py-6 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30"
          >
            <FileUp className="size-5 text-muted" />
            <span className="text-sm font-medium text-foreground">{file?.name ?? t.uploadPrompt}</span>
            <span className="text-xs text-muted">{t.uploadHint}</span>
          </button>
          {fieldErrors.proof_asset_id && <p className="text-xs text-danger">{fieldErrors.proof_asset_id}</p>}
        </div>
        </>
        )}
        {submitError !== "" && <p className="text-sm text-danger">{submitError}</p>}
      </FormModal>
    </AppShell>
  );
}
