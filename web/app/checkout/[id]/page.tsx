"use client";

import { use, useRef, useState } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Button, Card, Chip } from "@heroui/react";
import { CheckCircle2, Copy, CreditCard, FileUp, ShieldCheck, Wallet } from "lucide-react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { LinkButton } from "@/components/ui/link-button";
import { cn } from "@/lib/cn";
import { getProduct, getProgram, formatIDR, getBalance } from "@/lib/mock-data";
import { createOrder } from "@/lib/demo-store";
import { useSession } from "@/lib/session-context";
import { useCheckoutT } from "@/lib/i18n/dict/checkout";

type PaymentMethod = "manual" | "gateway" | "wallet";

export default function CheckoutPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const product = getProduct(id);
  const { user } = useSession();
  const t = useCheckoutT();
  const [method, setMethod] = useState<PaymentMethod>("manual");
  const [orderCode, setOrderCode] = useState<string | null>(null);
  const [fileName, setFileName] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  if (!product) notFound();
  const program = getProgram(product.programId);
  const balance = getBalance(user);
  const instantConfirm = method === "gateway" || method === "wallet";
  const isKecermatan = product.type === "kecermatan";

  if (orderCode) {
    return (
      <div className="flex min-h-dvh flex-col bg-background">
        <header className="flex items-center justify-between px-6 py-5">
          <Logo />
          <div className="flex items-center gap-2">
            <LanguageToggle />
            <ThemeToggle />
          </div>
        </header>
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
                  {t.orderCodeLabel} <span className="font-mono font-medium text-foreground">{orderCode}</span> —{" "}
                  {isKecermatan ? t.attemptsAddedInstant(product.includedAttempts!) : t.accessOpenInstant}{" "}
                  {t.confirmationSentMsg}
                  {method === "wallet" && t.walletUsedMsg(formatIDR(product.price), formatIDR(Math.max(0, balance)))}
                </>
              ) : (
                <>
                  {t.orderCodeLabel} <span className="font-mono font-medium text-foreground">{orderCode}</span> {t.pendingAdminMsg}{" "}
                  {isKecermatan ? t.attemptAddedPending : t.accessOpenPending}
                </>
              )}
            </p>
            <Card className="mt-6 gap-2 p-4 text-left">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted">{t.packageLabel}</span>
                <span className="font-medium text-foreground">{product.title}</span>
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
              href={isKecermatan ? `/student/kecermatan/${product.kecermatanTestId}` : "/student/tryouts"}
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

  return (
    <div className="min-h-dvh bg-background">
      <header className="flex items-center justify-between px-6 py-5">
        <Logo />
        <div className="flex items-center gap-2">
          <LanguageToggle />
          <ThemeToggle />
        </div>
      </header>

      <main className="mx-auto max-w-4xl px-5 pb-20">
        <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.pageTitle}</h1>
        <p className="mt-1 text-sm text-muted">{t.loggedInAs(user.name)}</p>

        <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_1.2fr]">
          <Card className="h-fit gap-3 p-5">
            <p className="text-xs font-medium text-muted uppercase">{t.orderSummary}</p>
            <div className="flex items-center gap-3">
              <div
                className="size-14 shrink-0 rounded-lg"
                style={{ background: `linear-gradient(135deg, ${product.coverFrom}, ${product.coverTo})` }}
              />
              <div>
                <p className="text-sm font-semibold text-foreground">{product.title}</p>
                {program && <p className="text-xs text-muted">{program.name}</p>}
              </div>
            </div>
            <div className="space-y-1.5 border-t border-border pt-3 text-sm">
              <div className="flex justify-between text-muted">
                <span>{t.normalPrice}</span>
                <span className="line-through">{formatIDR(product.originalPrice)}</span>
              </div>
              <div className="flex justify-between text-muted">
                <span>{t.discount}</span>
                <span className="text-success">
                  -{formatIDR(product.originalPrice - product.price)}
                </span>
              </div>
              <div className="flex justify-between border-t border-border pt-2 text-base font-semibold text-foreground">
                <span>{t.total}</span>
                <span>{formatIDR(product.price)}</span>
              </div>
            </div>
          </Card>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              const order = createOrder({
                userId: user.id,
                product,
                paymentMode: method,
                hasProof: method === "manual" && !!fileName,
              });
              setOrderCode(order.code);
            }}
            className="flex flex-col gap-5"
          >
            <div className="grid grid-cols-3 gap-2">
              {(
                [
                  { id: "manual", label: t.paymentMethodManual, icon: ShieldCheck },
                  { id: "gateway", label: t.paymentMethodGateway, icon: CreditCard },
                  { id: "wallet", label: t.paymentMethodWallet, icon: Wallet },
                ] as const
              ).map((m) => (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => setMethod(m.id)}
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
                  <div className="flex items-center justify-between rounded-lg border border-border bg-surface-secondary px-3.5 py-3">
                    <div>
                      <p className="text-xs text-muted">Bank Central — a.n. Taruna Eduka</p>
                      <p className="font-mono text-sm font-medium text-foreground">1234 5678 9012</p>
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      isIconOnly
                      onPress={() => navigator.clipboard?.writeText("1234567890912")}
                    >
                      <Copy className="size-3.5" />
                    </Button>
                  </div>
                </Card>

                <Card className="gap-3 p-5">
                  <p className="text-sm font-semibold text-foreground">{t.proofOfTransferTitle}</p>
                  <input
                    ref={fileRef}
                    type="file"
                    accept="image/*,.pdf"
                    className="hidden"
                    onChange={(e) => setFileName(e.target.files?.[0]?.name ?? null)}
                  />
                  <button
                    type="button"
                    onClick={() => fileRef.current?.click()}
                    className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30"
                  >
                    <FileUp className="size-5 text-muted" />
                    <span className="text-sm font-medium text-foreground">
                      {fileName ?? t.uploadPrompt}
                    </span>
                    <span className="text-xs text-muted">{t.uploadHint}</span>
                  </button>
                </Card>
              </>
            )}

            {method === "gateway" && (
              <Card className="gap-3 p-5">
                <div className="flex items-center gap-2">
                  <CreditCard className="size-4 text-accent" />
                  <p className="text-sm font-semibold text-foreground">{t.paymentMethodGateway}</p>
                </div>
                <p className="text-xs text-muted">{t.gatewayDesc}</p>
              </Card>
            )}

            {method === "wallet" && (
              <Card className="gap-3 p-5">
                <div className="flex items-center gap-2">
                  <Wallet className="size-4 text-accent" />
                  <p className="text-sm font-semibold text-foreground">{t.paymentMethodWallet}</p>
                </div>
                <div className="flex items-center justify-between rounded-lg border border-border bg-surface-secondary px-3.5 py-3 text-sm">
                  <span className="text-muted">{t.walletBalanceLabel}</span>
                  <span className="font-mono font-semibold text-foreground">{formatIDR(balance)}</span>
                </div>
                {balance < product.price ? (
                  <p className="text-xs text-danger">{t.insufficientBalance}</p>
                ) : (
                  <p className="text-xs text-muted">{t.remainingBalance(formatIDR(balance - product.price))}</p>
                )}
              </Card>
            )}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              fullWidth
              isDisabled={method === "manual" ? !fileName : method === "wallet" ? balance < product.price : false}
            >
              {method === "manual" && t.submitManual}
              {method === "gateway" && t.submitGateway}
              {method === "wallet" && t.submitWallet}
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
