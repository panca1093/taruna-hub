"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  catalogCrumb: "Katalog",
  backToCatalog: "Kembali ke katalog",
  chipIndividual: "Individual",
  chipBundle: (n: number) => `Bundle · ${n}x Tryout`,
  packageMultiHeading: (n: number) => `${n} tryout dalam paket ini`,
  packageSingleHeading: "Yang akan kamu kerjakan",
  subtestStat: (n: number, q: number) => `${n} subtes · ${q} soal`,
  totalMinStat: (min: number) => `${min} menit total`,
  workableUntil: (date: string) => `Dapat dikerjakan hingga ${date}`,
  totalDuration: (min: number) => `Total ${min} menit pengerjaan`,
  repeatable: "Dapat dikerjakan berulang kali selama masa aktif",
  discountLabel: (pct: number) => `Hemat ${pct}% dari harga normal`,
  ctaBuyNow: "Beli Sekarang",
  checkoutNote: "Pembayaran dan akses tryout mengikuti alur checkout.",
  loading: "Memuat produk…",
  loadErrorTitle: "Produk gagal dimuat",
  loadErrorDescription: "Periksa koneksi lalu coba lagi.",
  notFoundTitle: "Produk tidak tersedia",
  notFoundDescription: "Produk ini sudah tidak tampil di katalog.",
  retry: "Coba lagi",
};

const en: typeof id = {
  catalogCrumb: "Catalog",
  backToCatalog: "Back to catalog",
  chipIndividual: "Individual",
  chipBundle: (n: number) => `Bundle · ${n}x Tryout`,
  packageMultiHeading: (n: number) => `${n} tryouts in this package`,
  packageSingleHeading: "What you'll take",
  subtestStat: (n: number, q: number) => `${n} subtests · ${q} questions`,
  totalMinStat: (min: number) => `${min} min total`,
  workableUntil: (date: string) => `Available until ${date}`,
  totalDuration: (min: number) => `Total ${min} min to complete`,
  repeatable: "Can be retaken any number of times while active",
  discountLabel: (pct: number) => `Save ${pct}% off the normal price`,
  ctaBuyNow: "Buy Now",
  checkoutNote: "Payment and tryout access follow the checkout flow.",
  loading: "Loading product…",
  loadErrorTitle: "Product could not be loaded",
  loadErrorDescription: "Check the connection and try again.",
  notFoundTitle: "Product unavailable",
  notFoundDescription: "This product is no longer shown in the catalog.",
  retry: "Try again",
};

export function useCatalogDetailT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
