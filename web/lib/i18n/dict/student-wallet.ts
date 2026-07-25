"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Saldo",
  description: "Saldo tersimpan dan riwayat pemakaiannya",
  balanceLabel: "Saldo saat ini",
  topUpHint: "Top up saldo dilakukan oleh admin atau operator bimbel.",
  totalTopUp: "Total top up",
  totalSpent: "Total pemakaian",
  historyTitle: "Riwayat saldo",
  emptyTitle: "Belum ada transaksi saldo",
  emptyDescription: "Riwayat top up dan pemakaian saldomu akan muncul di sini.",
  colDate: "Tanggal",
  colDescription: "Keterangan",
  colType: "Tipe",
  colAmount: "Jumlah",
  typeLabel: { topup: "Top Up", deduction: "Pemakaian" },
};

const en: typeof id = {
  title: "Balance",
  description: "Your stored balance and how it has been used",
  balanceLabel: "Current balance",
  topUpHint: "Top-ups are performed by a bimbel admin or operator.",
  totalTopUp: "Total topped up",
  totalSpent: "Total spent",
  historyTitle: "Balance history",
  emptyTitle: "No balance activity yet",
  emptyDescription: "Your top-ups and purchases paid by balance will appear here.",
  colDate: "Date",
  colDescription: "Description",
  colType: "Type",
  colAmount: "Amount",
  typeLabel: { topup: "Top Up", deduction: "Spent" },
};

export function useStudentWalletT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
