"use client";

import Link from "next/link";
import { Activity, ClipboardList, MonitorPlay, Users, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { StatCard } from "@/components/stat-card";
import { BarChart, DonutStat } from "@/components/charts";
import { DataTable, type Column } from "@/components/data-table";
import { OrderStatusBadge } from "@/components/status-badge";
import { Card } from "@heroui/react";
import {
  allStudents,
  formatDate,
  formatIDR,
  getOrderTitle,
  getProduct,
  liveSessions,
  orders,
  programs,
  tryouts,
  findUser,
} from "@/lib/mock-data";
import { useSession } from "@/lib/session-context";
import type { Order } from "@/lib/types";
import { useAdminDashboardT } from "@/lib/i18n/dict/admin-dashboard";

const pending = orders.filter((o) => o.status === "pending");
const confirmed = orders.filter((o) => o.status === "confirmed");
const cancelled = orders.filter((o) => o.status === "cancelled");
const revenue = confirmed.reduce((sum, o) => sum + o.amount, 0);

const ordersByProgram = programs.map((p) => ({
  label: p.code,
  value: orders.filter((o) => getProduct(o.productId)?.programId === p.id).length,
}));

const recentOrders = [...orders].sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1)).slice(0, 6);

export default function AdminDashboardPage() {
  const { role } = useSession();
  const t = useAdminDashboardT();

  const columns: Column<Order>[] = [
    { key: "code", header: t.columns.code, render: (o) => <span className="font-mono text-xs text-muted">{o.code}</span> },
    { key: "student", header: t.columns.student, render: (o) => findUser(o.userId)?.name ?? o.userId },
    { key: "product", header: t.columns.product, render: (o) => getOrderTitle(o) },
    { key: "amount", header: t.columns.total, render: (o) => formatIDR(o.amount) },
    { key: "date", header: t.columns.date, render: (o) => formatDate(o.createdAt) },
    { key: "status", header: t.columns.status, render: (o) => <OrderStatusBadge status={o.status} /> },
  ];

  return (
    <AppShell title={t.title} description={t.description}>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label={t.totalStudents} value={String(allStudents.length)} icon={Users} delta={{ value: t.totalStudentsDelta, direction: "up" }} />
        <StatCard
          label={t.pendingOrders}
          value={String(pending.length)}
          icon={ClipboardList}
          ring={(pending.length / orders.length) * 100}
        />
        <StatCard label={t.activeTryouts} value={String(tryouts.length)} icon={Activity} />
        {role === "admin" ? (
          <StatCard label={t.confirmedRevenue} value={formatIDR(revenue)} icon={Wallet} delta={{ value: t.confirmedRevenueDelta, direction: "up" }} />
        ) : (
          <StatCard label={t.liveSessions} value={String(liveSessions.length)} icon={MonitorPlay} />
        )}
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-3">
        <Card className="gap-4 p-5 lg:col-span-2">
          <div>
            <p className="text-sm font-semibold text-foreground">{t.ordersByProgramTitle}</p>
            <p className="text-xs text-muted">{t.ordersByProgramDescription}</p>
          </div>
          <BarChart data={ordersByProgram} />
        </Card>
        <Card className="gap-4 p-5">
          <div>
            <p className="text-sm font-semibold text-foreground">{t.orderStatusTitle}</p>
            <p className="text-xs text-muted">{t.totalOrders(orders.length)}</p>
          </div>
          <DonutStat
            segments={[
              { label: t.confirmed, value: confirmed.length, className: "stroke-success" },
              { label: t.pending, value: pending.length, className: "stroke-warning" },
              { label: t.cancelled, value: cancelled.length, className: "stroke-danger" },
            ]}
          />
        </Card>
      </div>

      <Card className="mt-4 flex-row items-center justify-between gap-3 p-4">
        <div className="flex items-center gap-3">
          <span className="relative inline-flex size-2.5">
            <span className="absolute inline-flex size-full animate-ping rounded-full motion-reduce:animate-none bg-accent opacity-60" />
            <span className="relative inline-flex size-2.5 rounded-full bg-accent" />
          </span>
          <p className="text-sm text-foreground">
            <strong className="font-semibold">{t.liveNowStrong(liveSessions.length)}</strong> {t.liveNowSuffix}
          </p>
        </div>
        <Link href="/admin/monitoring" className="text-sm font-medium text-accent hover:underline">
          {t.viewMonitoring}
        </Link>
      </Card>

      <div className="mt-6">
        <div className="mb-3 flex items-center justify-between">
          <p className="text-sm font-semibold text-foreground">{t.recentOrders}</p>
          <Link href="/admin/orders" className="text-sm font-medium text-accent hover:underline">
            {t.viewAll}
          </Link>
        </div>
        <DataTable columns={columns} rows={recentOrders} rowKey={(o) => o.id} />
      </div>
    </AppShell>
  );
}
