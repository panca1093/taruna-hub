"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { CloudOff, Copy, CreditCard, FileUp, Receipt, TriangleAlert, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { cn } from "@/lib/cn";
import {
  api,
  ApiError,
  errorText,
  type BankTransfer,
  type CheckoutBody,
  type PublicProduct,
} from "@/lib/api/client";
import { formatIDR, isLessMoney, subtractMoney } from "@/lib/format-money";
import { useCheckoutT } from "@/lib/i18n/dict/checkout";

type PaymentMethod = CheckoutBody["payment_method"];
type LoadState = "loading" | "ready" | "missing" | "failed";
type Availability = "checking" | "ok" | "off";
type Problem = { text: string; href?: string; hrefLabel?: string };

const PROOF_TYPES = ["image/png", "image/jpeg", "image/webp"];

const newKey = () => crypto.randomUUID();

export default function CheckoutPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const t = useCheckoutT();
  const [product, setProduct] = useState<PublicProduct | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [balance, setBalance] = useState<string | null>(null);
  const [bank, setBank] = useState<BankTransfer | null>(null);
  const [bankState, setBankState] = useState<Availability>("checking");
  const [gateway, setGateway] = useState<Availability>("checking");
  const [method, setMethod] = useState<PaymentMethod>("wallet");
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [unresolved, setUnresolved] = useState(false);
  const [redirecting, setRedirecting] = useState(false);
  const [priceNotice, setPriceNotice] = useState<{ from: string; to: string } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  // One key per logical purchase: a retry of the same click finds its order instead of making another.
  const keyRef = useRef(newKey());
  const proofRef = useRef<{ file: File; assetId: string } | null>(null);

  const view: LoadState = state === "ready" && product?.id !== id ? "loading" : state;

  const loadPayments = useCallback(() => {
    let live = true;
    api.wallet
      .get()
      .then((wallet) => live && setBalance(wallet.balance))
      .catch(() => {});
    api.orders
      .gateway()
      .then(() => live && setGateway("ok"))
      .catch(() => live && setGateway("off"));
    api.orders
      .bankTransfer()
      .then((account) => {
        if (!live) return;
        setBank(account);
        setBankState("ok");
      })
      .catch(() => live && setBankState("off"));
    return () => {
      live = false;
    };
  }, []);

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
    const stop = loadPayments();
    return () => {
      cancelled = true;
      stop();
    };
  }, [id, reloadKey, loadPayments]);

  useEffect(() => {
    // Back from Midtrans can restore this page frozen in its redirecting state; the same key finds the same order.
    const onShow = (event: PageTransitionEvent) => {
      if (event.persisted) setRedirecting(false);
    };
    window.addEventListener("pageshow", onShow);
    return () => window.removeEventListener("pageshow", onShow);
  }, []);

  if (view !== "ready" || product === null) {
    return (
      <AppShell title={t.pageTitle} maxWidth="max-w-5xl">
        {view === "loading" ? (
          <div className="grid gap-6 lg:grid-cols-[1.6fr_1fr]">
            <div className="h-72 animate-pulse rounded-2xl border border-border bg-surface-secondary" />
            <div className="h-56 animate-pulse rounded-2xl border border-border bg-surface-secondary" />
          </div>
        ) : (
          <EmptyState
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
      </AppShell>
    );
  }

  const short = balance !== null && isLessMoney(balance, product.price);
  const locked = submitting || redirecting || unresolved;
  const manualOk = bankState === "ok" && bank !== null;
  const activeMethod: PaymentMethod = method === "gateway" && gateway !== "ok" ? "wallet" : method;

  const fresh = () => {
    keyRef.current = newKey();
    setUnresolved(false);
  };

  const choose = (next: PaymentMethod) => {
    if (locked) return;
    setMethod(next);
    setProblem(null);
    fresh();
  };

  const pickFile = (picked: File | null) => {
    setProblem(null);
    fresh();
    if (picked && !PROOF_TYPES.includes(picked.type)) {
      setFile(null);
      setFileError(t.uploadWrongType);
      return;
    }
    setFileError("");
    setFile(picked);
  };

  const findPendingOrder = async (): Promise<string | null> => {
    try {
      const page = await api.orders.list({ limit: 50 });
      return page.orders.find((order) => order.product_id === product.id && order.status === "pending")?.id ?? null;
    } catch {
      return null;
    }
  };

  const explain = async (err: unknown) => {
    const code = err instanceof ApiError ? err.code : undefined;
    if (err instanceof ApiError && err.fields?.proof_asset_id) {
      proofRef.current = null;
      setFileError(err.fields.proof_asset_id);
      fresh();
      setProblem({ text: errorText(err, t.submitFailed) });
      return;
    }
    if (code === "price_changed") {
      const from = product.price;
      const updated = await api.catalog.product(id).catch(() => null);
      if (updated) {
        setProduct(updated);
        setPriceNotice({ from, to: updated.price });
      }
      fresh();
      setProblem(null);
      return;
    }
    if (err instanceof ApiError && err.status === 404) {
      setState("missing");
      return;
    }
    if (code === "already_owned") {
      setProblem({ text: t.alreadyOwned, href: "/student/tryouts", hrefLabel: t.alreadyOwnedLink });
      return;
    }
    if (code === "order_pending") {
      const pendingId = await findPendingOrder();
      setProblem({
        text: t.orderPending,
        href: pendingId ? `/student/orders/${pendingId}` : "/student/orders",
        hrefLabel: pendingId ? t.orderPendingLink : t.orderPendingListLink,
      });
      return;
    }
    if (code === "insufficient_balance") {
      await api.wallet.get().then((wallet) => setBalance(wallet.balance)).catch(() => {});
      setProblem({ text: t.insufficientBalance, href: "/student/wallet", hrefLabel: t.topUpLink });
      return;
    }
    if (code === "service_unavailable") {
      loadPayments();
      setProblem({ text: t.serviceUnavailable });
      return;
    }
    const unknown = !(err instanceof ApiError) || err.status >= 500;
    if (unknown) {
      setUnresolved(true);
      setProblem({ text: t.unknownOutcome, href: "/student/orders", hrefLabel: t.unknownOutcomeOrders });
      return;
    }
    if (code === "conflict") fresh();
    setProblem({ text: errorText(err, t.submitFailed) });
  };

  const submit = async () => {
    setSubmitting(true);
    setProblem(null);
    setPriceNotice(null);
    try {
      let proofAssetId: string | undefined;
      if (activeMethod === "manual") {
        if (!file) return;
        if (proofRef.current?.file !== file) {
          try {
            const asset = await api.assets.uploadProof(file);
            proofRef.current = { file, assetId: asset.id };
          } catch (err) {
            setFileError(errorText(err, t.uploadFailed));
            return;
          }
        }
        proofAssetId = proofRef.current.assetId;
      }
      const created = await api.orders.checkout({
        product_id: product.id,
        payment_method: activeMethod,
        idempotency_key: keyRef.current,
        expected_price: product.price,
        ...(proofAssetId ? { proof_asset_id: proofAssetId } : {}),
      });
      if (created.payment_method === "gateway" && created.status === "pending" && created.redirect_url) {
        setRedirecting(true);
        window.location.assign(created.redirect_url);
        return;
      }
      router.replace(`/student/orders/${created.id}`);
    } catch (err) {
      await explain(err);
    } finally {
      setSubmitting(false);
    }
  };

  const walletBlocked = activeMethod === "wallet" && (balance === null || short);
  const manualBlocked = activeMethod === "manual" && (!file || !manualOk);
  const submitLabel = submitting
    ? t.submitting
    : redirecting
      ? t.redirecting
      : unresolved
        ? t.submitRetryUnknown
        : activeMethod === "wallet"
          ? t.submitWallet
          : activeMethod === "gateway"
            ? t.submitGateway
            : t.submitManual;

  const typeLabel =
    product.type === "kecermatan" ? t.typeKecermatan : product.type === "bundle" ? t.typeBundle : t.typeIndividual;

  return (
    <AppShell title={t.pageTitle} description={product.title} maxWidth="max-w-5xl">
      <nav className="mb-4 text-xs text-muted">
        <Link href={`/catalog/${product.id}`} className="hover:text-foreground hover:underline">
          {t.backToProduct}
        </Link>
      </nav>

      {priceNotice && (
        <div className="mb-4 flex items-start gap-3 rounded-xl border border-border bg-warning-soft p-4 text-sm text-warning-soft-foreground" role="alert">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" />
          <div>
            <p className="font-semibold">{t.priceChangedTitle}</p>
            <p className="mt-0.5">{t.priceChangedBody(formatIDR(priceNotice.from), formatIDR(priceNotice.to))}</p>
          </div>
        </div>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
        className="grid gap-6 lg:grid-cols-[1.6fr_1fr]"
      >
        <Card className="h-fit gap-4 rounded-2xl p-5 sm:p-6">
          <h2 className="font-display text-lg font-semibold text-foreground">{t.methodTitle}</h2>
          <div role="radiogroup" aria-label={t.methodTitle} className="flex flex-col gap-3">
            <MethodOption
              selected={activeMethod === "wallet"}
              disabled={locked}
              onSelect={() => choose("wallet")}
              icon={Wallet}
              title={t.methodWallet}
              description={balance === null ? t.methodWalletUnknown : t.methodWalletAvailable(formatIDR(balance))}
            />
            {gateway === "ok" && (
              <MethodOption
                selected={activeMethod === "gateway"}
                disabled={locked}
                onSelect={() => choose("gateway")}
                icon={CreditCard}
                title={t.methodGateway}
                description={t.methodGatewayDesc}
              />
            )}
            <MethodOption
              selected={activeMethod === "manual"}
              disabled={locked || !manualOk}
              onSelect={() => choose("manual")}
              icon={Receipt}
              title={t.methodManual}
              description={
                bankState === "checking" ? t.methodManualLoading : manualOk ? t.methodManualDesc : t.methodManualUnavailable
              }
            />
          </div>

          {activeMethod === "wallet" && (
            <div className="rounded-xl border border-border bg-surface-secondary p-4 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-muted">{t.walletBalanceLabel}</span>
                <span className="font-mono font-semibold text-foreground">{balance === null ? "—" : formatIDR(balance)}</span>
              </div>
              {balance === null ? (
                <p className="mt-2 text-xs text-danger">{t.balanceUnknown}</p>
              ) : short ? (
                <p className="mt-2 text-xs text-danger">
                  {t.insufficientBalance}{" "}
                  <Link href="/student/wallet" className="font-medium underline">
                    {t.topUpLink}
                  </Link>
                </p>
              ) : (
                <p className="mt-2 text-xs text-muted">{t.remainingBalance(formatIDR(subtractMoney(balance, product.price)))}</p>
              )}
            </div>
          )}

          {activeMethod === "gateway" && (
            <div className="rounded-xl border border-border bg-surface-secondary p-4 text-sm text-muted">{t.gatewayDesc}</div>
          )}

          {activeMethod === "manual" && manualOk && (
            <div className="flex flex-col gap-4">
              <div className="rounded-xl border border-border bg-surface-secondary p-4">
                <p className="text-sm font-semibold text-foreground">{t.bankTitle}</p>
                <p className="mt-1 text-xs text-muted">{t.bankDesc(formatIDR(product.price))}</p>
                <div className="mt-3 flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-xs text-muted">
                      {bank.bank_name} · {t.bankHolder(bank.account_holder)}
                    </p>
                    <p className="font-mono text-base font-semibold break-all text-foreground">{bank.account_number}</p>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    isIconOnly
                    aria-label={t.copyAccount}
                    onPress={() => {
                      void navigator.clipboard?.writeText(bank.account_number).then(() => toast.success(t.copied));
                    }}
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
              </div>

              <div>
                <p className="text-sm font-semibold text-foreground">{t.proofTitle}</p>
                <input
                  ref={fileRef}
                  type="file"
                  accept={PROOF_TYPES.join(",")}
                  className="hidden"
                  onChange={(e) => {
                    pickFile(e.target.files?.[0] ?? null);
                    e.target.value = "";
                  }}
                />
                <button
                  type="button"
                  disabled={locked}
                  onClick={() => fileRef.current?.click()}
                  className="mt-2 flex w-full flex-col items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center transition-colors hover:border-accent hover:bg-accent-soft disabled:opacity-60"
                >
                  <FileUp className="size-5 text-muted" />
                  <span className="text-sm font-medium break-all text-foreground">{file?.name ?? t.uploadPrompt}</span>
                  <span className="text-xs text-muted">{file ? t.uploadChange : t.uploadHint}</span>
                </button>
                {fileError !== "" && (
                  <p className="mt-2 text-sm text-danger" role="alert">
                    {fileError}
                  </p>
                )}
              </div>
            </div>
          )}

          {activeMethod === "manual" && bankState === "off" && (
            <p className="text-sm text-danger">{t.methodManualUnavailable}</p>
          )}

          <p className="rounded-xl bg-surface-secondary px-4 py-3 text-xs text-muted">{t.accessNote}</p>
        </Card>

        <div className="flex flex-col gap-4 lg:sticky lg:top-6 lg:h-fit">
          <Card className="gap-3 rounded-2xl p-5 sm:p-6">
            <h2 className="font-display text-lg font-semibold text-foreground">{t.summaryTitle}</h2>
            <div>
              <p className="text-sm font-semibold text-foreground">{product.title}</p>
              <p className="text-xs text-muted">
                {product.program_name} · {typeLabel}
              </p>
            </div>
            {product.type === "bundle" && <p className="text-xs text-muted">{t.includedTryouts(product.tryouts.length)}</p>}
            {product.type === "kecermatan" && product.included_attempts !== null && (
              <p className="text-xs text-muted">{t.includedAttempts(product.included_attempts)}</p>
            )}
            {product.active_until && (
              <p className="text-xs text-muted">{t.validUntil(new Date(product.active_until).toLocaleDateString())}</p>
            )}
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

            {problem && (
              <div className="rounded-lg border border-border bg-danger-soft p-3 text-sm text-danger-soft-foreground" role="alert">
                <p>{problem.text}</p>
                {problem.href && (
                  <Link href={problem.href} className="mt-1 inline-block font-medium underline">
                    {problem.hrefLabel}
                  </Link>
                )}
              </div>
            )}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              fullWidth
              isDisabled={submitting || redirecting || (!unresolved && (walletBlocked || manualBlocked))}
            >
              {submitLabel}
            </Button>
            <p className="text-center text-xs text-muted">
              {t.termsAgreementPrefix}{" "}
              <Link href="#" className="text-accent hover:underline">
                {t.termsLink}
              </Link>
              .
            </p>
          </Card>
        </div>
      </form>
    </AppShell>
  );
}

function MethodOption({
  selected,
  disabled,
  onSelect,
  icon: Icon,
  title,
  description,
}: {
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      disabled={disabled}
      onClick={onSelect}
      className={cn(
        "flex items-center gap-3 rounded-xl border px-4 py-3.5 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-60",
        selected ? "border-accent bg-accent-soft" : "border-border hover:border-accent",
      )}
    >
      <span
        className={cn(
          "inline-flex size-4 shrink-0 items-center justify-center rounded-full border",
          selected ? "border-accent" : "border-border",
        )}
        aria-hidden
      >
        {selected && <span className="size-2 rounded-full bg-accent" />}
      </span>
      <Icon className="size-5 shrink-0 text-muted" />
      <span className="min-w-0">
        <span className="block text-sm font-semibold text-foreground">{title}</span>
        <span className="block text-xs text-muted">{description}</span>
      </span>
    </button>
  );
}
