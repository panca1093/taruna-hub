"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Chip } from "@heroui/react";
import { ArrowDownLeft, ArrowUpRight, CloudOff, Wallet as WalletIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { TopupStatusBadge } from "@/components/status-badge";
import { LinkButton } from "@/components/ui/link-button";
import { cn } from "@/lib/cn";
import { api, errorText, type Topup, type Wallet, type WalletEntry } from "@/lib/api/client";
import { formatIDR, formatSignedIDR } from "@/lib/format-money";
import { useStudentWalletT } from "@/lib/i18n/dict/student-wallet";
import { useWalletTopupT } from "@/lib/i18n/dict/wallet-topup";

const PAGE_SIZE = 50;

const when = (iso: string) => new Date(iso).toLocaleString();

export default function StudentWalletPage() {
  const t = useStudentWalletT();
  const topupT = useWalletTopupT();
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [entries, setEntries] = useState<WalletEntry[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [topups, setTopups] = useState<Topup[]>([]);
  useEffect(() => {
    let cancelled = false;
    Promise.all([api.wallet.get(), api.wallet.entries({ limit: PAGE_SIZE }), api.topups.list({ limit: PAGE_SIZE })])
      .then(([head, page, requests]) => {
        if (cancelled) return;
        setWallet(head);
        setEntries(page.entries);
        setTotal(page.total);
        setTopups(requests.topups);
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
          <LinkButton href="/student/wallet/topup" variant="primary" size="sm" className="mt-3">
            {t.topUpButton}
          </LinkButton>
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
                    <Link href={`/student/wallet/topups/${topup.id}`} className="block min-w-0 rounded-md hover:underline" aria-label={`${topup.code} — ${topupT.historyView}`}>
                      <p className="font-mono text-xs text-muted">
                        {topup.code} · {when(topup.created_at)}
                      </p>
                      <p className="font-medium text-foreground">{formatIDR(topup.amount)}</p>
                      {topup.status === "rejected" && (
                        <p className="text-xs text-danger">{t.rejectReason(topup.reject_reason)}</p>
                      )}
                    </Link>
                    <TopupStatusBadge status={topup.status} />
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
                              <Link href={`/student/wallet/topups/${entry.source_id}`} className="ml-1 font-mono text-xs text-accent hover:underline">
                                {topupCodes.get(entry.source_id)}
                              </Link>
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

    </AppShell>
  );
}
