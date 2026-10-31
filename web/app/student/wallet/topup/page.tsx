"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@heroui/react";
import { CloudOff, Copy, FileUp } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { TextInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { api, ApiError, errorText, type BankTransfer } from "@/lib/api/client";
import { formatIDR, isLessMoney } from "@/lib/format-money";
import { useWalletTopupT } from "@/lib/i18n/dict/wallet-topup";

type Method = "gateway" | "manual";

const PRESETS = ["25000", "50000", "100000"];

const newKey = () => crypto.randomUUID();

export default function StudentTopupPage() {
  const t = useWalletTopupT();
  const router = useRouter();
  const [minTopup, setMinTopup] = useState<string | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [gatewayOk, setGatewayOk] = useState<boolean | null>(null);
  const [bank, setBank] = useState<BankTransfer | null>(null);
  const [bankFailed, setBankFailed] = useState(false);
  const [method, setMethod] = useState<Method>("manual");
  const [amount, setAmount] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const keyRef = useRef(newKey());
  const lockRef = useRef(false);
  const proofRef = useRef<{ file: File; assetId: string } | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.topups
      .list({ limit: 1 })
      .then((page) => {
        if (!cancelled) setMinTopup(page.min_topup);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    api.orders
      .gateway()
      .then(() => {
        if (cancelled) return;
        setGatewayOk(true);
        setMethod("gateway");
      })
      .catch(() => {
        if (!cancelled) setGatewayOk(false);
      });
    api.orders
      .bankTransfer()
      .then((account) => {
        if (!cancelled) setBank(account);
      })
      .catch(() => {
        if (!cancelled) setBankFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const changed = () => {
    setSubmitError("");
    setFieldErrors({});
    keyRef.current = newKey();
  };

  const amountValid = /^[1-9]\d*$/.test(amount);
  const belowMin = amountValid && minTopup !== null && isLessMoney(amount, minTopup);
  const minText = minTopup === null ? "" : formatIDR(minTopup);
  const amountError =
    fieldErrors.amount ?? (amount !== "" && !amountValid ? t.amountRequired : belowMin ? t.amountBelowMin(minText) : undefined);
  const manualReady = method === "manual" && file !== null && bank !== null;
  const canSubmit = amountValid && !belowMin && minTopup !== null && (method === "gateway" || manualReady);

  const submit = async () => {
    if (lockRef.current || !canSubmit || (method === "manual" && !file)) return;
    lockRef.current = true;
    setSubmitting(true);
    setFieldErrors({});
    setSubmitError("");
    try {
      let created;
      if (method === "gateway") {
        created = await api.topups.create({ method: "gateway", amount, idempotency_key: keyRef.current });
      } else {
        if (proofRef.current?.file !== file) {
          const asset = await api.assets.uploadProof(file as File);
          proofRef.current = { file: file as File, assetId: asset.id };
        }
        created = await api.topups.create({
          amount,
          idempotency_key: keyRef.current,
          proof_asset_id: proofRef.current.assetId,
        });
      }
      // replace, so Back from the status page never returns to a filled form that could create a second top-up
      router.replace(`/student/wallet/topups/${created.id}`);
    } catch (err) {
      if (err instanceof ApiError && err.fields?.proof_asset_id) proofRef.current = null;
      if (err instanceof ApiError && err.fields) setFieldErrors(err.fields);
      setSubmitError(errorText(err, t.submitFailed));
      lockRef.current = false;
      setSubmitting(false);
    }
  };

  const crumbs = (
    <nav aria-label="breadcrumb" className="text-xs text-muted">
      <Link href="/student/wallet" className="hover:text-foreground">
        {t.breadcrumbBalance}
      </Link>
    </nav>
  );

  if (loadFailed) {
    return (
      <AppShell title={t.topupTitle} description={t.topupDescription} maxWidth="max-w-5xl">
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" size="sm" onPress={() => {
                setLoadFailed(false);
                setReloadKey((key) => key + 1);
              }}>
              {t.retry}
            </Button>
          }
        />
      </AppShell>
    );
  }

  const methods: { id: Method; label: string; hint: string }[] = [
    ...(gatewayOk ? [{ id: "gateway" as const, label: t.methodGateway, hint: t.methodGatewayHint }] : []),
    { id: "manual", label: t.methodManual, hint: t.methodManualHint },
  ];

  return (
    <AppShell title={t.topupTitle} description={t.topupDescription} maxWidth="max-w-5xl" actions={crumbs}>
      <div className="grid gap-6 lg:grid-cols-[1fr_20rem] lg:items-start">
        <div className="space-y-8 rounded-xl border border-border bg-surface p-5 sm:p-6">
          <section className="space-y-4">
            <h2 className="text-base font-semibold text-foreground">{t.amountSection}</h2>
            <div className="grid grid-cols-3 gap-2">
              {PRESETS.map((preset) => {
                const tooLow = minTopup !== null && isLessMoney(preset, minTopup);
                return (
                  <button
                    key={preset}
                    type="button"
                    disabled={tooLow || minTopup === null}
                    aria-pressed={amount === preset}
                    onClick={() => {
                      setAmount(preset);
                      changed();
                    }}
                    className={cn(
                      "rounded-xl border px-3 py-2.5 text-sm font-medium transition-colors disabled:opacity-50",
                      amount === preset
                        ? "border-accent bg-accent-soft text-accent-soft-foreground"
                        : "border-border text-foreground hover:border-accent/40",
                    )}
                  >
                    {formatIDR(preset)}
                  </button>
                );
              })}
            </div>
            <TextInput
              label={t.amountOther}
              description={minTopup === null ? undefined : t.amountMinimum(minText)}
              inputMode="numeric"
              autoComplete="off"
              value={amount}
              error={amountError}
              onChange={(e) => {
                setAmount(e.target.value.replace(/\D/g, "").replace(/^0+/, ""));
                changed();
              }}
            />
          </section>

          <section className="space-y-3">
            <h2 className="text-base font-semibold text-foreground">{t.methodSection}</h2>
            <div role="radiogroup" aria-label={t.methodSection} className="space-y-2">
              {methods.map((m) => (
                <label
                  key={m.id}
                  className={cn(
                    "flex cursor-pointer items-start gap-3 rounded-xl border px-4 py-3 transition-colors focus-within:ring-2 focus-within:ring-focus",
                    method === m.id ? "border-accent bg-accent-soft/40" : "border-border hover:border-accent/40",
                  )}
                >
                  <input
                    type="radio"
                    name="method"
                    className="mt-1 accent-accent"
                    checked={method === m.id}
                    onChange={() => {
                      setMethod(m.id);
                      changed();
                    }}
                  />
                  <span>
                    <span className="block text-sm font-medium text-foreground">{m.label}</span>
                    <span className="block text-xs text-muted">{m.hint}</span>
                  </span>
                </label>
              ))}
            </div>

            {method === "manual" && (
              <div className="space-y-5 pt-2">
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
                        aria-label={t.copyAccount}
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
                      changed();
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
              </div>
            )}
            <p className="rounded-lg bg-surface-secondary px-4 py-3 text-xs text-muted">{t.creditNote}</p>
          </section>
        </div>

        <aside className="space-y-4 rounded-xl border border-border bg-surface p-5 lg:sticky lg:top-6">
          <h2 className="text-base font-semibold text-foreground">{t.summaryTitle}</h2>
          <div className="space-y-2 text-sm">
            <p className="text-muted">{t.summaryLine}</p>
            <div className="flex justify-between gap-3">
              <span className="text-muted">{t.summaryAmount}</span>
              <span className="font-medium text-foreground">{amountValid ? formatIDR(amount) : "—"}</span>
            </div>
          </div>
          <div className="flex justify-between gap-3 border-t border-border pt-3">
            <span className="font-semibold text-foreground">{t.summaryTotal}</span>
            <span className="font-display text-lg font-semibold text-foreground">{amountValid ? formatIDR(amount) : "—"}</span>
          </div>
          {submitError !== "" && (
            <p role="alert" className="text-sm text-danger">
              {submitError}
            </p>
          )}
          <Button variant="primary" fullWidth isDisabled={!canSubmit || submitting} onPress={submit}>
            {submitting ? t.submitting : method === "gateway" ? t.continuePay : t.submitManual}
          </Button>
        </aside>
      </div>
    </AppShell>
  );
}
