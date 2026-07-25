"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Dashboard",
  description: "Ringkasan operasional hari ini",
  totalStudents: "Total siswa",
  totalStudentsDelta: "+4 minggu ini",
  pendingOrders: "Pesanan menunggu konfirmasi",
  activeTryouts: "Tryout aktif",
  confirmedRevenue: "Pendapatan terkonfirmasi",
  confirmedRevenueDelta: "+12% bulan ini",
  liveSessions: "Sesi ujian berlangsung",
  ordersByProgramTitle: "Pesanan per program",
  ordersByProgramDescription: "Distribusi seluruh pesanan berdasarkan jenjang",
  orderStatusTitle: "Status pesanan",
  totalOrders: (n: number) => `Total ${n} pesanan`,
  confirmed: "Terkonfirmasi",
  pending: "Menunggu",
  cancelled: "Dibatalkan",
  liveNowStrong: (n: number) => `${n} siswa`,
  liveNowSuffix: "sedang mengerjakan tryout sekarang",
  viewMonitoring: "Lihat monitoring →",
  recentOrders: "Pesanan terbaru",
  viewAll: "Lihat semua →",
  columns: {
    code: "Kode",
    student: "Siswa",
    product: "Produk",
    total: "Total",
    date: "Tanggal",
    status: "Status",
  },
};

const en: typeof id = {
  title: "Dashboard",
  description: "Today's operational summary",
  totalStudents: "Total students",
  totalStudentsDelta: "+4 this week",
  pendingOrders: "Orders awaiting confirmation",
  activeTryouts: "Active tryouts",
  confirmedRevenue: "Confirmed revenue",
  confirmedRevenueDelta: "+12% this month",
  liveSessions: "Live exam sessions",
  ordersByProgramTitle: "Orders by program",
  ordersByProgramDescription: "Distribution of all orders by level",
  orderStatusTitle: "Order status",
  totalOrders: (n: number) => `Total ${n} orders`,
  confirmed: "Confirmed",
  pending: "Pending",
  cancelled: "Cancelled",
  liveNowStrong: (n: number) => `${n} students`,
  liveNowSuffix: "taking a tryout right now",
  viewMonitoring: "View monitoring →",
  recentOrders: "Recent orders",
  viewAll: "View all →",
  columns: {
    code: "Code",
    student: "Student",
    product: "Product",
    total: "Total",
    date: "Date",
    status: "Status",
  },
};

export function useAdminDashboardT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
