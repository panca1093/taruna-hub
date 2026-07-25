"use client";

import Link from "next/link";
import { Receipt } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { OrderStatusBadge } from "@/components/status-badge";
import { useSession } from "@/lib/session-context";
import { formatDate, formatIDR, getOrdersForUser, getOrderTitle, TOPUP_PRODUCT_ID } from "@/lib/mock-data";
import { useStudentOrdersT } from "@/lib/i18n/dict/student-orders";

export default function StudentOrdersPage() {
  const { user } = useSession();
  const t = useStudentOrdersT();
  const orders = getOrdersForUser(user.id).sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1));

  return (
    <AppShell title={t.title} description={t.description} maxWidth="max-w-6xl">
      {orders.length === 0 ? (
        <EmptyState icon={Receipt} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <div className="overflow-hidden rounded-xl border border-border bg-surface">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-surface-secondary text-left text-[11px] font-medium tracking-wide text-muted uppercase">
                <th className="px-4 py-2.5">{t.colCode}</th>
                <th className="px-4 py-2.5">{t.colProduct}</th>
                <th className="px-4 py-2.5">{t.colDate}</th>
                <th className="px-4 py-2.5">{t.colTotal}</th>
                <th className="px-4 py-2.5">{t.colMethod}</th>
                <th className="px-4 py-2.5">{t.colStatus}</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.id} className="border-b border-border last:border-b-0">
                  <td className="px-4 py-3 font-mono text-xs text-muted">{o.code}</td>
                  <td className="px-4 py-3 text-foreground">
                    {o.productId === TOPUP_PRODUCT_ID ? (
                      getOrderTitle(o)
                    ) : (
                      <Link href={`/catalog/${o.productId}`} className="hover:underline">
                        {getOrderTitle(o)}
                      </Link>
                    )}
                  </td>
                  <td className="px-4 py-3 text-muted">{formatDate(o.createdAt)}</td>
                  <td className="px-4 py-3 font-medium text-foreground">{formatIDR(o.amount)}</td>
                  <td className="px-4 py-3 text-muted">{t.paymentModeLabel[o.paymentMode]}</td>
                  <td className="px-4 py-3">
                    <OrderStatusBadge status={o.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </AppShell>
  );
}
