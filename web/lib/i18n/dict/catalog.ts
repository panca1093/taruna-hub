"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Katalog",
  description: "Paket individual berisi satu tryout. Paket bundle berisi beberapa tryout dengan harga lebih hemat.",
  publicEyebrow: "Katalog",
  publicHeading: "Pilih paket tryout sesuai jenjangmu",
  filterAllProgram: "Semua program",
  filterAllType: "Semua tipe",
  typeIndividual: "Individual",
  typeBundle: "Bundle",
  searchLabel: "Cari paket",
  searchPlaceholder: "Cari judul paket…",
  loading: "Memuat katalog…",
  loadErrorTitle: "Katalog gagal dimuat",
  loadErrorDescription: "Periksa koneksi lalu coba lagi.",
  retry: "Coba lagi",
  emptyTitle: "Belum ada paket yang cocok",
  emptyDescription: "Coba pilih program lain, ubah kata kunci, atau kembali lagi nanti.",
  discountChip: (pct: number) => `Hemat ${pct}%`,
  bundleSize: (n: number) => `${n} tryout`,
};

const en: typeof id = {
  title: "Catalog",
  description: "Individual packages include one tryout. Bundle packages include several tryouts at a lower price.",
  publicEyebrow: "Catalog",
  publicHeading: "Choose the right package for your level",
  filterAllProgram: "All programs",
  filterAllType: "All types",
  typeIndividual: "Individual",
  typeBundle: "Bundle",
  searchLabel: "Search packages",
  searchPlaceholder: "Search package title…",
  loading: "Loading catalog…",
  loadErrorTitle: "The catalog could not be loaded",
  loadErrorDescription: "Check the connection and try again.",
  retry: "Try again",
  emptyTitle: "No package matches yet",
  emptyDescription: "Pick another program, change the keyword, or check back later.",
  discountChip: (pct: number) => `Save ${pct}%`,
  bundleSize: (n: number) => `${n} tryouts`,
};

export function useCatalogT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
