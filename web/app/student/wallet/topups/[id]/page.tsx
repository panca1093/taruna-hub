"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { ArrowRight, Check, Clock, CloudOff, SearchX, X } from "lucide-react";
import { Icon } from "@/components/icon";
import { NoteBox, Panel, TextLink } from "@/components/page-blocks";
import { Breadcrumb } from "@/components/page-heading";
import { cn } from "@/lib/cn";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { TopupStatusBadge } from "@/components/status-badge";
import { LinkButton } from "@/components/ui/link-button";
import { api, ApiError, errorText, type Topup } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useStudentWalletT } from "@/lib/i18n/dict/student-wallet";
import { useWalletTopupT } from "@/lib/i18n/dict/wallet-topup";

const POLL_MS = 5000;

const when = (iso: string) => new Date(iso).toLocaleString();

export default function StudentTopupStatusPage() {
  const t = useWalletTopupT();
  const walletT = useStudentWalletT();
  const { id } = useParams<{ id: string }>();
  const [topup, setTopup] = useState<Topup | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "missing" | "failed">("loading");
  const [refreshing, setRefreshing] = useState(false);
  const [refreshFailed, setRefreshFailed] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const alive = useRef(true);
  const loaded = useRef(false);

  const fetchTopup = useCallback(
    () =>
      api.topups.get(id).then(
        (fresh) => {
          if (!alive.current) return;
          loaded.current = true;
          setTopup(fresh);
          setState("ready");
          setRefreshFailed(false);
        },
        (err: unknown) => {
          if (!alive.current) return;
          if (err instanceof ApiError && err.status === 404) setState("missing");
          else if (loaded.current) setRefreshFailed(true);
          else setState("failed");
        },
      ),
    [id],
  );

  useEffect(() => {
    alive.current = true;
    fetchTopup();
    return () => {
      alive.current = false;
    };
  }, [fetchTopup]);

  const pending = topup?.status === "pending";
  useEffect(() => {
    if (!pending) return;
    const timer = setInterval(fetchTopup, POLL_MS);
    return () => clearInterval(timer);
  }, [pending, fetchTopup]);

  const refresh = async () => {
    setRefreshing(true);
    await fetchTopup();
    if (alive.current) setRefreshing(false);
  };

  const cancel = async () => {
    setCancelling(true);
    try {
      setTopup(await api.topups.cancel(id));
    } catch (err) {
      toast.danger(t.cancelFailed, { description: errorText(err, t.loadErrorDescription) });
      fetchTopup();
    } finally {
      if (alive.current) setCancelling(false);
    }
  };

  const shell = (children: React.ReactNode, titled = true) => (
    <AppShell title={titled ? t.statusTitle : undefined} maxWidth="max-w-5xl">
      {children}
    </AppShell>
  );

  if (state === "loading") return shell(<p className="py-14 text-center text-small text-muted">{t.loading}</p>);
  if (state === "missing")
    return shell(
      <EmptyState
        icon={SearchX}
        title={t.notFoundTitle}
        description={t.notFoundDescription}
        action={
          <LinkButton href="/student/wallet" variant="secondary">
            {t.statusBack}
          </LinkButton>
        }
      />,
    );
  if (state === "failed" || topup === null)
    return shell(
      <EmptyState
        icon={CloudOff}
        title={t.statusLoadFailed}
        description={t.loadErrorDescription}
        action={
          <Button variant="primary" onPress={refresh}>
            {t.retry}
          </Button>
        }
      />,
    );

  const gatewayPending = topup.status === "pending" && topup.method === "gateway";
  const tone = topup.status === "approved" ? "success" : topup.status === "rejected" ? "failed" : "pending";
  const bannerTitle = topup.status === "approved" ? t.approvedTitle : topup.status === "rejected" ? t.rejectedTitle : t.pendingTitle;
  const bannerBody =
    topup.status === "approved"
      ? t.approvedDescription
      : topup.status === "rejected"
        ? `${t.rejectedReason}: ${topup.reject_reason === "" ? t.rejectedNoReason : topup.reject_reason}`
        : topup.method === "gateway"
          ? t.pendingGateway
          : t.pendingManual;

  return shell(
    <>
      <Breadcrumb
        items={[
          { label: t.breadcrumbDashboard, href: "/student" },
          { label: t.breadcrumbBalance, href: "/student/wallet" },
          { label: t.statusTitle },
        ]}
      />
      <div
        role="status"
        aria-live="polite"
        className={cn(
          "mb-6 flex items-center gap-4 rounded-menu p-5 sm:p-6",
          tone === "success" && "bg-success-soft text-success-soft-foreground",
          tone === "failed" && "bg-peach",
          tone === "pending" && "bg-accent-soft",
        )}
      >
        <span className="inline-flex size-12 shrink-0 items-center justify-center rounded-full bg-surface text-foreground">
          <Icon icon={tone === "success" ? Check : tone === "failed" ? X : Clock} size="md" />
        </span>
        <div className="min-w-0">
          <h1 className="font-display text-hero text-balance">{bannerTitle}</h1>
          <p className="mt-2 text-caption text-foreground/80">{bannerBody}</p>
        </div>
      </div>

      <div className="grid gap-7 lg:grid-cols-detail lg:items-start">
        <Panel>
          <h2 className="mb-3 font-display text-heading-sm">{t.statusTitle}</h2>
          {topup.status === "approved" && (
            <p className="text-caption leading-relaxed text-muted">
              {t.ledgerRef}: {walletT.sourceLabel.topup} <span className="font-mono text-label">{topup.code}</span>
            </p>
          )}
          {refreshFailed && (
            <p role="alert" className="mt-3 text-label text-danger">
              {t.refreshFailed}
            </p>
          )}
          <div className="mt-6 flex flex-wrap items-center gap-2.5">
            {gatewayPending && topup.redirect_url && (
              <LinkButton href={topup.redirect_url} variant="primary">
                {t.continuePayment}
                <Icon icon={ArrowRight} size="sm" />
              </LinkButton>
            )}
            {pending && (
              <Button variant="secondary" onPress={refresh} isDisabled={refreshing}>
                {refreshing ? t.refreshing : t.refresh}
              </Button>
            )}
            {gatewayPending && (
              <Button variant="ghost" onPress={cancel} isDisabled={cancelling}>
                {cancelling ? t.cancelling : t.cancelTopup}
              </Button>
            )}
            {topup.status === "approved" && (
              <LinkButton href="/student/wallet" variant="primary">
                {t.viewLedger}
                <Icon icon={ArrowRight} size="sm" />
              </LinkButton>
            )}
            {topup.status === "rejected" && (
              <LinkButton href="/student/wallet/topup" variant="primary">
                {t.newTopup}
                <Icon icon={ArrowRight} size="sm" />
              </LinkButton>
            )}
          </div>
          {gatewayPending && <NoteBox className="mt-5">{t.cancelHint}</NoteBox>}
          <TextLink href="/student/wallet" className="mt-5">
            {t.statusBack}
          </TextLink>
        </Panel>

        <Panel as="aside">
          <p className="font-mono text-label text-muted">{topup.code}</p>
          <p className="mt-3 font-display text-heading">{formatIDR(topup.amount)}</p>
          <dl className="mt-3 text-caption">
            <SummaryLine label={t.statusMethod}>{t.statusMethodLabel[topup.method] ?? topup.method}</SummaryLine>
            <SummaryLine label={walletT.colStatus}>
              <TopupStatusBadge status={topup.status} />
            </SummaryLine>
            <SummaryLine label={t.statusCreated}>{when(topup.created_at)}</SummaryLine>
            {topup.approved_at && <SummaryLine label={t.approvedAt}>{when(topup.approved_at)}</SummaryLine>}
            {topup.rejected_at && <SummaryLine label={t.rejectedAt}>{when(topup.rejected_at)}</SummaryLine>}
          </dl>
        </Panel>
      </div>
    </>,
    false,
  );
}

function SummaryLine({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-5 py-3">
      <dt className="text-muted">{label}</dt>
      <dd className="text-right font-semibold text-foreground">{children}</dd>
    </div>
  );
}
