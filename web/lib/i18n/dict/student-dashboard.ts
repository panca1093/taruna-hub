"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  greeting: (name: string) => `Halo, ${name}`,
  subtitle: "Ringkasan aktivitas belajarmu",
  ownedTryouts: "Tryout dimiliki",
  completedTryouts: "Tryout selesai",
  averageScore: "Skor rata-rata",
  balance: "Saldo",
  emptyTitle: "Belum ada tryout yang aktif",
  emptyDescription: "Beli paket tryout dari katalog untuk mulai berlatih.",
  viewCatalog: "Lihat Katalog",
  tryoutStatus: "Status tryout",
  ownedCountDescription: (n: number) => `Dari ${n} tryout yang kamu miliki`,
  segmentCompleted: "Selesai",
  segmentInProgress: "Sedang berjalan",
  segmentNotStarted: "Belum dikerjakan",
  latestResults: "Hasil terbaru",
  latestResultsDescription: "Percobaan tryout yang sudah kamu selesaikan",
  viewAll: "Lihat semua →",
  noResults: "Belum ada hasil tryout.",
  attempt: (n: number) => `Percobaan ${n}`,
  myTryouts: "Tryout Saya",
  orderHistory: "Riwayat Pesanan",
};

const en: typeof id = {
  greeting: (name: string) => `Hi, ${name}`,
  subtitle: "Summary of your learning activity",
  ownedTryouts: "Tryouts owned",
  completedTryouts: "Tryouts completed",
  averageScore: "Average score",
  balance: "Balance",
  emptyTitle: "No active tryouts yet",
  emptyDescription: "Buy a tryout package from the catalog to start practicing.",
  viewCatalog: "View Catalog",
  tryoutStatus: "Tryout status",
  ownedCountDescription: (n: number) => `Of ${n} tryouts you own`,
  segmentCompleted: "Completed",
  segmentInProgress: "In progress",
  segmentNotStarted: "Not started",
  latestResults: "Latest results",
  latestResultsDescription: "Tryout attempts you've completed",
  viewAll: "View all →",
  noResults: "No tryout results yet.",
  attempt: (n: number) => `Attempt ${n}`,
  myTryouts: "My Tryouts",
  orderHistory: "Order History",
};

export function useStudentDashboardT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
