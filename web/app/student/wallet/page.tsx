"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Chip } from "@heroui/react";
import { ArrowDownLeft, ArrowUpRight, CloudOff, Wallet as WalletIcon } from "lucide-react";
import { Icon } from "@/components/icon";
import { Panel } from "@/components/page-blocks";
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
    <AppShell
      title={t.title}
      description={t.description}
      maxWidth="max-w-5xl"
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.title }]}
      actions={
        <LinkButton href="/student/wallet/topup" variant="primary">
          {t.topUpButton}
        </LinkButton>
      }
    >
      <div className="grid gap-7 lg:grid-cols-wallet lg:items-start">
        <div className="min-w-0 space-y-6 lg:order-1">
          {!loadFailed && entries !== null && (
            <Panel padding="none">
              <h2 className="px-6 pt-6 pb-2 font-display text-heading-sm sm:px-7 sm:pt-7">{t.requestsTitle}</h2>
              {topups.length === 0 ? (
                <p className="px-6 pb-6 text-caption text-muted sm:px-7 sm:pb-7">{t.requestsEmpty}</p>
              ) : (
                <ul className="px-6 pb-2 sm:px-7">
                  {topups.map((topup) => (
                    <li key={topup.id} className="flex items-center justify-between gap-4 border-b border-border py-4 text-caption last:border-b-0">
                      <Link
                        href={`/student/wallet/topups/${topup.id}`}
                        className="block min-w-0 rounded-chip outline-none hover:underline focus-visible:ring-2 focus-visible:ring-focus"
                        aria-label={`${topup.code} — ${topupT.historyView}`}
                      >
                        <p className="font-display text-title text-foreground">{formatIDR(topup.amount)}</p>
                        <p className="mt-1 font-mono text-label text-muted">
                          {topup.code} · {when(topup.created_at)}
                        </p>
                        {topup.status === "rejected" && <p className="mt-1 text-label text-danger">{t.rejectReason(topup.reject_reason)}</p>}
                      </Link>
                      <TopupStatusBadge status={topup.status} />
                    </li>
                  ))}
                </ul>
              )}
            </Panel>
          )}

          {loadFailed ? (
            <EmptyState
              icon={CloudOff}
              title={t.loadErrorTitle}
              description={t.loadErrorDescription}
              action={
                <Button variant="primary" onPress={retry}>
                  {t.retry}
                </Button>
              }
            />
          ) : entries === null ? (
            <p className="py-14 text-center text-small text-muted">{t.loading}</p>
          ) : entries.length === 0 ? (
            <EmptyState icon={WalletIcon} title={t.emptyTitle} description={t.emptyDescription} />
          ) : (
            <>
              <div className="overflow-hidden rounded-card border border-border bg-surface">
                <h2 className="px-6 pt-6 pb-4 font-display text-heading-sm sm:px-7 sm:pt-7">{t.historyTitle}</h2>
                <div className="overflow-x-auto">
                  <table className="w-full text-caption">
                    <thead>
                      <tr className="border-y border-border bg-background text-left text-label font-medium text-muted">
                        <th className="px-6 py-3 font-medium sm:px-7">{t.colDate}</th>
                        <th className="px-4 py-3 font-medium">{t.colType}</th>
                        <th className="px-4 py-3 font-medium">{t.colDescription}</th>
                        <th className="px-4 py-3 font-medium">{t.colSource}</th>
                        <th className="px-4 py-3 text-right font-medium">{t.colAmount}</th>
                        <th className="px-6 py-3 text-right font-medium sm:px-7">{t.colBalanceAfter}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {entries.map((entry) => {
                        const credit = !entry.amount.startsWith("-");
                        return (
                          <tr key={entry.id} className="border-b border-border last:border-b-0">
                            <td className="px-6 py-4 whitespace-nowrap text-muted sm:px-7">{when(entry.created_at)}</td>
                            <td className="px-4 py-4">
                              <Chip
                                size="sm"
                                className={cn(
                                  "font-medium",
                                  credit ? "bg-success-soft text-success-soft-foreground" : "bg-surface-secondary text-muted",
                                )}
                              >
                                <Icon icon={credit ? ArrowDownLeft : ArrowUpRight} size="xs" />
                                {t.typeLabel[entry.kind] ?? entry.kind}
                              </Chip>
                            </td>
                            <td className="px-4 py-4 text-foreground">{entry.reason === "" ? "—" : entry.reason}</td>
                            <td className="px-4 py-4 text-muted">
                              {t.sourceLabel[entry.source_type] ?? entry.source_type}
                              {entry.source_type === "topup" && topupCodes.get(entry.source_id) && (
                                <Link
                                  href={`/student/wallet/topups/${entry.source_id}`}
                                  className="ml-1 rounded-chip font-mono text-label text-accent outline-none hover:underline focus-visible:ring-2 focus-visible:ring-focus"
                                >
                                  {topupCodes.get(entry.source_id)}
                                </Link>
                              )}
                            </td>
                            <td
                              className={cn(
                                "px-4 py-4 text-right font-mono font-medium whitespace-nowrap",
                                credit ? "text-success" : "text-foreground",
                              )}
                            >
                              {formatSignedIDR(entry.amount)}
                            </td>
                            <td className="px-6 py-4 text-right font-mono whitespace-nowrap text-foreground sm:px-7">
                              {formatIDR(entry.balance_after)}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
              {moreFailed !== "" && <p className="text-center text-small text-danger">{moreFailed}</p>}
              {entries.length < total && (
                <div className="flex justify-center">
                  <Button variant="secondary" onPress={loadMore} isDisabled={loadingMore}>
                    {loadingMore ? t.loadingMore : t.loadMore}
                  </Button>
                </div>
              )}
            </>
          )}
        </div>

        <Panel as="aside" className="lg:sticky lg:top-6 lg:order-2">
          <p className="mb-3 text-label text-muted">{t.balanceLabel}</p>
          <p className="font-display text-metric-lg text-foreground">{wallet === null ? "—" : formatIDR(wallet.balance)}</p>
          <p className="mt-1 text-label leading-relaxed text-muted">{t.topUpHint}</p>
          <div className="mt-5 grid grid-cols-2 gap-3 border-t border-border pt-5">
            <div>
              <p className="text-label text-muted">{t.totalCredits}</p>
              <p className="mt-1 text-small font-medium text-success">{wallet === null ? "—" : formatIDR(wallet.total_credits)}</p>
            </div>
            <div>
              <p className="text-label text-muted">{t.totalDebits}</p>
              <p className="mt-1 text-small font-medium text-foreground">{wallet === null ? "—" : formatIDR(wallet.total_debits)}</p>
            </div>
          </div>
        </Panel>
      </div>
    </AppShell>
  );
}
