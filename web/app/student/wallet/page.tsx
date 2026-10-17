"use client";

import { useEffect, useReducer } from "react";
import { Card, Chip } from "@heroui/react";
import { ArrowDownLeft, ArrowUpRight, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { cn } from "@/lib/cn";
import { formatDateTime, formatIDR, getBalance, getWalletTransactionsForUser } from "@/lib/mock-data";
import { STORE_EVENT } from "@/lib/demo-store";
import { useUser } from "@/lib/session-context";
import { useStudentWalletT } from "@/lib/i18n/dict/student-wallet";

export default function StudentWalletPage() {
  const user = useUser();
  const t = useStudentWalletT();
  const [, forceUpdate] = useReducer((n: number) => n + 1, 0);

  useEffect(() => {
    window.addEventListener(STORE_EVENT, forceUpdate);
    return () => window.removeEventListener(STORE_EVENT, forceUpdate);
  }, []);

  const transactions = getWalletTransactionsForUser(user.id);
  const balance = getBalance(user);

  const totalTopUp = transactions.filter((w) => w.type === "topup").reduce((s, w) => s + w.amount, 0);
  const totalSpent = transactions.filter((w) => w.type === "deduction").reduce((s, w) => s + w.amount, 0);

  return (
    <AppShell title={t.title} description={t.description} maxWidth="max-w-4xl">
      <div className="space-y-6">
        <Card className="gap-4 p-5">
          <div className="flex items-start justify-between">
            <span className="inline-flex size-9 items-center justify-center rounded-lg bg-accent-soft text-accent-soft-foreground">
              <Wallet className="size-[18px]" strokeWidth={2} />
            </span>
          </div>
          <div>
            <p className="font-display text-3xl font-semibold tracking-tight text-foreground">{formatIDR(balance)}</p>
            <p className="mt-0.5 text-sm text-muted">{t.balanceLabel}</p>
          </div>
          <p className="text-xs text-muted">{t.topUpHint}</p>
          <div className="grid grid-cols-2 gap-3 border-t border-border pt-3 text-sm">
            <div>
              <p className="text-xs text-muted">{t.totalTopUp}</p>
              <p className="font-medium text-success">{formatIDR(totalTopUp)}</p>
            </div>
            <div>
              <p className="text-xs text-muted">{t.totalSpent}</p>
              <p className="font-medium text-foreground">{formatIDR(totalSpent)}</p>
            </div>
          </div>
        </Card>

        {transactions.length === 0 ? (
          <EmptyState icon={Wallet} title={t.emptyTitle} description={t.emptyDescription} />
        ) : (
          <div className="overflow-hidden rounded-xl border border-border bg-surface">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-surface-secondary text-left text-[11px] font-medium tracking-wide text-muted uppercase">
                  <th className="px-4 py-2.5">{t.colDate}</th>
                  <th className="px-4 py-2.5">{t.colDescription}</th>
                  <th className="px-4 py-2.5">{t.colType}</th>
                  <th className="px-4 py-2.5 text-right">{t.colAmount}</th>
                </tr>
              </thead>
              <tbody>
                {transactions.map((w) => (
                  <tr key={w.id} className="border-b border-border last:border-b-0">
                    <td className="px-4 py-3 text-muted">{formatDateTime(w.createdAt)}</td>
                    <td className="px-4 py-3 text-foreground">{w.description}</td>
                    <td className="px-4 py-3">
                      <Chip
                        size="sm"
                        className={cn(
                          "font-medium",
                          w.type === "topup"
                            ? "bg-success-soft text-success-soft-foreground"
                            : "bg-surface-secondary text-muted",
                        )}
                      >
                        {w.type === "topup" ? (
                          <ArrowDownLeft className="size-3" />
                        ) : (
                          <ArrowUpRight className="size-3" />
                        )}
                        {t.typeLabel[w.type]}
                      </Chip>
                    </td>
                    <td
                      className={cn(
                        "px-4 py-3 text-right font-mono font-medium",
                        w.type === "topup" ? "text-success" : "text-foreground",
                      )}
                    >
                      {w.type === "topup" ? "+" : "−"}
                      {formatIDR(w.amount)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </AppShell>
  );
}
