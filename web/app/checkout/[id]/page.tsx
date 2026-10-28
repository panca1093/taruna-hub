"use client";

import { use, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip } from "@heroui/react";
import { CheckCircle2, CloudOff, Copy, FileUp, ShieldCheck, Wallet } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { LinkButton } from "@/components/ui/link-button";
import { cn } from "@/lib/cn";
import {
  api,
  ApiError,
  errorText,
  type BankTransfer,
  type Order,
  type PublicProduct,
} from "@/lib/api/client";
import { formatIDR, isLessMoney, subtractMoney } from "@/lib/format-money";
import { useUser } from "@/lib/session-context";
import { useCheckoutT } from "@/lib/i18n/dict/checkout";

// The gateway joins in P14; until then only these two are offered.
type PaymentMethod = "manual" | "wallet";
type LoadState = "loading" | "ready" | "missing" | "failed";

const newKey = () => crypto.randomUUID();

export default function CheckoutPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const user = useUser();
  const t = useCheckoutT();
  const [product, setProduct] = useState<PublicProduct | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [balance, setBalance] = useState<string | null>(null);
  const [bank, setBank] = useState<BankTransfer | null>(null);
  const [bankFailed, setBankFailed] = useState(false);
  const [method, setMethod] = useState<PaymentMethod>("manual");
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [order, setOrder] = useState<Order | null>(null);
  const [balanceAfter, setBalanceAfter] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  // One key per purchase attempt: a retry of the same click finds its order instead of making another.
  const keyRef = useRef(newKey());
  const proofRef = useRef<{ file: File; assetId: string } | null>(null);

  const view: LoadState = state === "ready" && product?.id !== id ? "loading" : state;

  useEffect(() => {
    let cancelled = false;
    api.catalog
      .product(id)
      .then((loaded) => {
        if (cancelled) return;
        setProduct(loaded);
        setState("ready");
      })
      .catch((err) => {
        if (cancelled) return;
        setState(err instanceof ApiError && err.status === 404 ? "missing" : "failed");
      });
    api.wallet
      .get()
      .then((wallet) => {
        if (!cancelled) setBalance(wallet.balance);
      })
      .catch(() => {});
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
  }, [id, reloadKey]);

  const header = (
    <header className="flex items-center justify-between px-6 py-5">
      <Logo />
      <div className="flex items-center gap-2">
        <LanguageToggle />
        <ThemeToggle />
      </div>
    </header>
  );

  if (view !== "ready" || product === null) {
    return (
      <div className="min-h-dvh bg-background">
        {header}
        <main className="mx-auto max-w-4xl px-5 pb-20">
          {view === "loading" ? (
            <div className="mt-8 h-72 animate-pulse rounded-xl border border-border bg-surface-secondary" />
          ) : (
            <EmptyState
              className="mt-10"
              icon={CloudOff}
              title={view === "missing" ? t.notFoundTitle : t.loadErrorTitle}
              description={view === "missing" ? t.notFoundDescription : t.loadErrorDescription}
              action={
                view === "missing" ? (
                  <LinkButton href="/catalog" variant="primary" size="sm">
                    {t.backToCatalog}
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
        </main>
      </div>
    );
  }

  const instantConfirm = order?.status === "confirmed";
  const isKecermatan = order?.product_type === "kecermatan";
  const short = balance !== null && isLessMoney(balance, product.price);

  if (order) {
    return (
      <div className="flex min-h-dvh flex-col bg-background">
        {header}
        <main className="flex flex-1 items-center justify-center px-5 pb-16">
          <div className="w-full max-w-sm text-center">
            <span className="mx-auto inline-flex size-14 items-center justify-center rounded-full bg-success-soft text-success-soft-foreground">
              <CheckCircle2 className="size-7" />
            </span>
            <h1 className="font-display mt-4 text-xl font-semibold text-foreground">
              {instantConfirm ? t.paymentSuccessTitle : t.orderCreatedTitle}
            </h1>
            <p className="mt-2 text-sm text-muted">
              {instantConfirm ? (
                <>
                  {t.orderCodeLabel} <span className="font-mono font-medium text-foreground">{order.code}</span> —{" "}
                  {isKecermatan ? t.attemptsAddedInstant(order.included_attempts ?? 0) : t.accessOpenInstant} {t.confirmationSentMsg}
                  {order.payment_method === "wallet" &&
                    balanceAfter !== null &&
                    t.walletUsedMsg(formatIDR(order.price), formatIDR(balanceAfter))}
                </>
              ) : (
                <>
                  {t.orderCodeLabel} <span className="font-mono font-medium text-foreground">{order.code}</span>{" "}
                  {t.pendingAdminMsg} {isKecermatan ? t.attemptAddedPending : t.accessOpenPending}
                </>
              )}
            </p>
            <Card className="mt-6 gap-2 p-4 text-left">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted">{t.packageLabel}</span>
                <span className="font-medium text-foreground">{order.product_title}</span>
              </div>
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted">{t.statusLabel}</span>
                <Chip
                  size="sm"
                  className={cn(
                    "font-medium",
                    instantConfirm ? "bg-success-soft text-success-soft-foreground" : "bg-warning-soft text-warning-soft-foreground",
                  )}
                >
                  {instantConfirm ? t.statusConfirmed : t.statusPendingConfirm}
                </Chip>
              </div>
            </Card>
            <LinkButton
              href={isKecermatan ? `/student/kecermatan/${order.kecermatan_test_id}` : "/student/tryouts"}
              variant="primary"
              size="lg"
              className="mt-6 w-full"
            >
              {isKecermatan ? t.viewKecermatanTest : t.viewMyPackages}
            </LinkButton>
          </div>
        </main>
      </div>
    );
  }

  const choose = (next: PaymentMethod) => {
    setMethod(next);
    setSubmitError("");
    keyRef.current = newKey();
  };

  const submit = async () => {
    setSubmitting(true);
    setSubmitError("");
    try {
      let proofAssetId: string | undefined;
      if (method === "manual") {
        if (!file) return;
        if (proofRef.current?.file !== file) {
          const asset = await api.assets.uploadProof(file);
          proofRef.current = { file, assetId: asset.id };
        }
        proofAssetId = proofRef.current.assetId;
      }
      const created = await api.orders.checkout({
        product_id: product.id,
        payment_method: method,
        idempotency_key: keyRef.current,
        expected_price: product.price,
        ...(proofAssetId ? { proof_asset_id: proofAssetId } : {}),
      });
      if (created.payment_method === "wallet") {
        setBalanceAfter(await api.wallet.get().then((wallet) => wallet.balance).catch(() => null));
      }
      setOrder(created);
    } catch (err) {
      if (err instanceof ApiError && err.fields?.proof_asset_id) proofRef.current = null;
      if (err instanceof ApiError && err.code === "price_changed") {
        await api.catalog.product(id).then(setProduct).catch(() => {});
      }
      setSubmitError(
        err instanceof ApiError && err.code === "already_owned"
          ? t.alreadyOwned
          : err instanceof ApiError && err.code === "price_changed"
            ? t.priceChanged
            : err instanceof ApiError && err.code === "order_pending"
              ? t.orderPending
              : err instanceof ApiError && err.code === "insufficient_balance"
            ? t.insufficientBalance
            : errorText(err, t.submitFailed),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-dvh bg-background">
      {header}

      <main className="mx-auto max-w-4xl px-5 pb-20">
        <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.pageTitle}</h1>
        <p className="mt-1 text-sm text-muted">{t.loggedInAs(user.name)}</p>

        <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_1.2fr]">
          <Card className="h-fit gap-3 p-5">
            <p className="text-xs font-medium text-muted uppercase">{t.orderSummary}</p>
            <div className="flex items-center gap-3">
              <div className="size-14 shrink-0 rounded-lg bg-gradient-ring" />
              <div>
                <p className="text-sm font-semibold text-foreground">{product.title}</p>
                <p className="text-xs text-muted">{product.program_name}</p>
              </div>
            </div>
            <div className="space-y-1.5 border-t border-border pt-3 text-sm">
              {product.original_price && (
                <>
                  <div className="flex justify-between text-muted">
                    <span>{t.normalPrice}</span>
                    <span className="line-through">{formatIDR(product.original_price)}</span>
                  </div>
                  <div className="flex justify-between text-muted">
                    <span>{t.discount}</span>
                    <span className="text-success">-{formatIDR(subtractMoney(product.original_price, product.price))}</span>
                  </div>
                </>
              )}
              <div className="flex justify-between border-t border-border pt-2 text-base font-semibold text-foreground">
                <span>{t.total}</span>
                <span>{formatIDR(product.price)}</span>
              </div>
            </div>
          </Card>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
            className="flex flex-col gap-5"
          >
            <div className="grid grid-cols-2 gap-2">
              {(
                [
                  { id: "manual", label: t.paymentMethodManual, icon: ShieldCheck },
                  { id: "wallet", label: t.paymentMethodWallet, icon: Wallet },
                ] as const
              ).map((m) => (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => choose(m.id)}
                  className={cn(
                    "flex flex-col items-center gap-1.5 rounded-xl border px-3 py-3 text-xs font-medium transition-colors",
                    method === m.id
                      ? "border-accent bg-accent-soft text-accent-soft-foreground"
                      : "border-border text-muted hover:border-accent/40",
                  )}
                >
                  <m.icon className="size-4" />
                  {m.label}
                </button>
              ))}
            </div>

            {method === "manual" && (
              <>
                <Card className="gap-3 p-5">
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="size-4 text-accent" />
                    <p className="text-sm font-semibold text-foreground">{t.bankTransferTitle}</p>
                  </div>
                  <p className="text-xs text-muted">
                    {t.bankTransferDescPrefix} <strong className="text-foreground">{formatIDR(product.price)}</strong>{" "}
                    {t.bankTransferDescSuffix}
                  </p>
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
                </Card>

                <Card className="gap-3 p-5">
                  <p className="text-sm font-semibold text-foreground">{t.proofOfTransferTitle}</p>
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
                    className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30"
                  >
                    <FileUp className="size-5 text-muted" />
                    <span className="text-sm font-medium text-foreground">{file?.name ?? t.uploadPrompt}</span>
                    <span className="text-xs text-muted">{t.uploadHint}</span>
                  </button>
                </Card>
              </>
            )}

            {method === "wallet" && (
              <Card className="gap-3 p-5">
                <div className="flex items-center gap-2">
                  <Wallet className="size-4 text-accent" />
                  <p className="text-sm font-semibold text-foreground">{t.paymentMethodWallet}</p>
                </div>
                <div className="flex items-center justify-between rounded-lg border border-border bg-surface-secondary px-3.5 py-3 text-sm">
                  <span className="text-muted">{t.walletBalanceLabel}</span>
                  <span className="font-mono font-semibold text-foreground">
                    {balance === null ? "—" : formatIDR(balance)}
                  </span>
                </div>
                {balance !== null &&
                  (short ? (
                    <p className="text-xs text-danger">{t.insufficientBalance}</p>
                  ) : (
                    <p className="text-xs text-muted">
                      {t.remainingBalance(formatIDR(subtractMoney(balance, product.price)))}
                    </p>
                  ))}
              </Card>
            )}

            {submitError !== "" && <p className="text-sm text-danger">{submitError}</p>}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              fullWidth
              isDisabled={submitting || (method === "manual" ? !file || !bank : balance === null || short)}
            >
              {submitting ? t.submitting : method === "manual" ? t.submitManual : t.submitWallet}
            </Button>
            <p className="text-center text-xs text-muted">
              {t.termsAgreementPrefix}{" "}
              <Link href="#" className="text-accent hover:underline">
                {t.termsLink}
              </Link>
              .
            </p>
          </form>
        </div>
      </main>
    </div>
  );
}
