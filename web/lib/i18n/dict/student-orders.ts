"use client";

import { useLocale } from "@/lib/i18n/locale-context";
import type { PaymentMode } from "@/lib/types";

const id = {
  title: "Riwayat Pesanan",
  description: "Semua transaksi pembelian paket tryoutmu",
  emptyTitle: "Belum ada pesanan",
  emptyDescription: "Pesananmu akan muncul di sini setelah kamu membeli paket tryout.",
  colCode: "Kode",
  colProduct: "Produk",
  colDate: "Tanggal",
  colTotal: "Total",
  colMethod: "Metode",
  colStatus: "Status",
  paymentModeLabel: {
    gateway: "Gateway",
    manual: "Transfer Manual",
    wallet: "Saldo",
  } as Record<PaymentMode, string>,
};

const en: typeof id = {
  title: "Order History",
  description: "All your tryout package purchase transactions",
  emptyTitle: "No orders yet",
  emptyDescription: "Your orders will appear here after you buy a tryout package.",
  colCode: "Code",
  colProduct: "Product",
  colDate: "Date",
  colTotal: "Total",
  colMethod: "Method",
  colStatus: "Status",
  paymentModeLabel: {
    gateway: "Gateway",
    manual: "Bank Transfer",
    wallet: "Balance",
  } as Record<PaymentMode, string>,
};

export function useStudentOrdersT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
