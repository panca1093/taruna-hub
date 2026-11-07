"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Rekap Pendapatan",
  description: "Pesanan terkonfirmasi dan top-up disetujui menurut hari Asia/Jakarta. Laporan saja, bukan penyelesaian dana.",
  from: "Dari tanggal",
  to: "Sampai tanggal",
  apply: "Terapkan",
  applying: "Memuat…",
  loading: "Memuat rekap…",
  loadFailed: "Rekap gagal dimuat.",
  retry: "Coba lagi",
  rangeInvalid: "Rentang tanggal tidak valid (maksimal 366 hari).",
  statOrders: "Pesanan terkonfirmasi",
  statOrdersTotal: "Total pesanan",
  statTopups: "Top-up disetujui",
  statTopupsTotal: "Total top-up",
  topupsNote: "Pesanan dengan saldo dibayar dari top-up sebelumnya, jadi jangan dijumlahkan.",
  byMethod: "Per metode bayar",
  byProduct: "Per jenis produk",
  colGroup: "Kelompok",
  colDate: "Tanggal",
  colOrders: "Pesanan",
  colTopups: "Top-up",
  colCount: "Jumlah",
  colTotal: "Total",
  noActivity: "Tidak ada transaksi pada rentang ini.",
  loadMoreDays: "Muat hari berikutnya",
  loadingMore: "Memuat…",
  method: { manual: "Transfer manual", wallet: "Saldo", gateway: "Gateway" } as Record<string, string>,
  product: { individual: "Individual", bundle: "Paket", kecermatan: "Kecermatan" } as Record<string, string>,
};

const en: typeof id = {
  title: "Revenue Recap",
  description: "Confirmed orders and approved top-ups by Asia/Jakarta day. A report only, not a settlement.",
  from: "From",
  to: "To",
  apply: "Apply",
  applying: "Loading…",
  loading: "Loading the recap…",
  loadFailed: "The recap couldn't be loaded.",
  retry: "Try again",
  rangeInvalid: "Invalid date range (at most 366 days).",
  statOrders: "Confirmed orders",
  statOrdersTotal: "Orders total",
  statTopups: "Approved top-ups",
  statTopupsTotal: "Top-ups total",
  topupsNote: "Wallet orders are paid from earlier top-ups, so don't add the two together.",
  byMethod: "By payment method",
  byProduct: "By product type",
  colGroup: "Group",
  colDate: "Date",
  colOrders: "Orders",
  colTopups: "Top-ups",
  colCount: "Count",
  colTotal: "Total",
  noActivity: "No transactions in this range.",
  loadMoreDays: "Load more days",
  loadingMore: "Loading…",
  method: { manual: "Manual transfer", wallet: "Wallet", gateway: "Gateway" },
  product: { individual: "Individual", bundle: "Bundle", kecermatan: "Kecermatan" },
};

export function useReportT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
