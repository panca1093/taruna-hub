"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { Button, toast } from "@heroui/react";
import { CloudOff, SearchX } from "lucide-react";
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

  const shell = (children: React.ReactNode) => (
    <AppShell title={t.statusTitle} maxWidth="max-w-3xl">
      {children}
    </AppShell>
  );

  if (state === "loading") return shell(<p className="py-14 text-center text-sm text-muted">{t.loading}</p>);
  if (state === "missing")
    return shell(
      <EmptyState
        icon={SearchX}
        title={t.notFoundTitle}
        description={t.notFoundDescription}
        action={
          <LinkButton href="/student/wallet" variant="secondary" size="sm">
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
          <Button variant="primary" size="sm" onPress={refresh}>
            {t.retry}
          </Button>
        }
      />,
    );

  const gatewayPending = topup.status === "pending" && topup.method === "gateway";

  return shell(
    <div className="space-y-6">
      <div className="space-y-4 rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="font-mono text-sm text-muted">{topup.code}</p>
          <TopupStatusBadge status={topup.status} />
        </div>
        <p className="font-display text-3xl font-semibold tracking-tight text-foreground">{formatIDR(topup.amount)}</p>

        <div role="status" className="rounded-lg bg-surface-secondary px-4 py-3 text-sm">
          {topup.status === "pending" && (
            <>
              <p className="font-medium text-foreground">{t.pendingTitle}</p>
              <p className="mt-1 text-muted">{topup.method === "gateway" ? t.pendingGateway : t.pendingManual}</p>
            </>
          )}
          {topup.status === "approved" && (
            <>
              <p className="font-medium text-success">{t.approvedTitle}</p>
              <p className="mt-1 text-muted">{t.approvedDescription}</p>
            </>
          )}
          {topup.status === "rejected" && (
            <>
              <p className="font-medium text-danger">{t.rejectedTitle}</p>
              <p className="mt-1 text-muted">
                {t.rejectedReason}: {topup.reject_reason === "" ? t.rejectedNoReason : topup.reject_reason}
              </p>
            </>
          )}
        </div>

        <dl className="grid grid-cols-2 gap-3 border-t border-border pt-4 text-sm">
          <div>
            <dt className="text-xs text-muted">{t.statusMethod}</dt>
            <dd className="text-foreground">{t.statusMethodLabel[topup.method] ?? topup.method}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted">{t.statusCreated}</dt>
            <dd className="text-foreground">{when(topup.created_at)}</dd>
          </div>
          {topup.approved_at && (
            <div>
              <dt className="text-xs text-muted">{t.approvedAt}</dt>
              <dd className="text-foreground">{when(topup.approved_at)}</dd>
            </div>
          )}
          {topup.rejected_at && (
            <div>
              <dt className="text-xs text-muted">{t.rejectedAt}</dt>
              <dd className="text-foreground">{when(topup.rejected_at)}</dd>
            </div>
          )}
          {topup.status === "approved" && (
            <div className="col-span-2">
              <dt className="text-xs text-muted">{t.ledgerRef}</dt>
              <dd className="text-foreground">
                {walletT.sourceLabel.topup} <span className="font-mono text-xs">{topup.code}</span>
              </dd>
            </div>
          )}
        </dl>

        {refreshFailed && (
          <p role="alert" className="text-xs text-danger">
            {t.refreshFailed}
          </p>
        )}

        <div className="flex flex-wrap gap-2 border-t border-border pt-4">
          {gatewayPending && topup.redirect_url && (
            <LinkButton href={topup.redirect_url} variant="primary" size="sm">
              {t.continuePayment}
            </LinkButton>
          )}
          {pending && (
            <Button variant="secondary" size="sm" onPress={refresh} isDisabled={refreshing}>
              {refreshing ? t.refreshing : t.refresh}
            </Button>
          )}
          {gatewayPending && (
            <Button variant="ghost" size="sm" onPress={cancel} isDisabled={cancelling}>
              {cancelling ? t.cancelling : t.cancelTopup}
            </Button>
          )}
          {topup.status === "approved" && (
            <LinkButton href="/student/wallet" variant="primary" size="sm">
              {t.viewLedger}
            </LinkButton>
          )}
          {topup.status === "rejected" && (
            <LinkButton href="/student/wallet/topup" variant="primary" size="sm">
              {t.newTopup}
            </LinkButton>
          )}
        </div>
        {gatewayPending && <p className="text-xs text-muted">{t.cancelHint}</p>}
      </div>
      <Link href="/student/wallet" className="text-sm text-accent hover:underline">
        {t.statusBack}
      </Link>
    </div>,
  );
}
