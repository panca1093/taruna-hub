"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  catalogCrumb: "Katalog",
  backToCatalog: "Kembali ke katalog",
  chipKecermatan: "Tes Kecermatan",
  chipIndividual: "Individual",
  chipBundle: (n: number) => `Bundle · ${n}x Tryout`,
  howItWorksTitle: "Cara kerja tes ini",
  kecermatanMechanic: (options: number) =>
    `Setiap kolom menampilkan ${options} item referensi berlabel. Set ini tetap sama untuk seluruh soal di kolom itu — tiap soal menampilkan ulang sebagian item dengan tepat satu item dihilangkan, dan kamu memilih label mana yang hilang.`,
  kolomSoalStat: (kolom: number, soal: number) => `${kolom} kolom · ${soal} soal/kolom`,
  kolomDurationStat: (kolom: number, min: number) => `${kolom} kolom · ${min} menit/kolom`,
  perMinuteStat: (min: number) => `${min} menit/kolom`,
  itemType: {
    char: "Kode huruf",
    number: "Kode angka",
    symbol: "Simbol",
    emoji: "Emoji",
    image: "Gambar",
    shape: "Bentuk",
    mix: "Campuran",
  } as Record<string, string>,
  noProctor:
    "Tidak ada mode layar penuh, pencatatan pelanggaran, atau ranking untuk format ini — hanya latihan kecepatan dan ketelitian yang dinilai per attempt.",
  ctaKecermatanRemaining: (n: number) => `${n} attempt tersisa — Kerjakan`,
  ctaKecermatanExhausted: "Kuota Habis — Beli Attempt Tambahan",
  attemptsPerPurchase: (n: number) => `${n} kali kesempatan mengerjakan per pembelian`,
  validUntil: (date: string) => `Kesempatan berlaku hingga ${date}`,
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
  chipKecermatan: "Kecermatan Test",
  chipIndividual: "Individual",
  chipBundle: (n: number) => `Bundle · ${n}x Tryout`,
  howItWorksTitle: "How this test works",
  kecermatanMechanic: (options: number) =>
    `Each kolom shows ${options} labeled reference items. This set stays the same for every soal in that kolom — each soal shows a subset with exactly one item removed, and you pick which label is missing.`,
  kolomSoalStat: (kolom: number, soal: number) => `${kolom} kolom · ${soal} soal/kolom`,
  kolomDurationStat: (kolom: number, min: number) => `${kolom} kolom · ${min} min/kolom`,
  perMinuteStat: (min: number) => `${min} min/kolom`,
  itemType: {
    char: "Letter code",
    number: "Number code",
    symbol: "Symbol",
    emoji: "Emoji",
    image: "Image",
    shape: "Shape",
    mix: "Mixed",
  } as Record<string, string>,
  noProctor:
    "No fullscreen mode, violation logging, or ranking for this format — only speed and accuracy are scored per attempt.",
  ctaKecermatanRemaining: (n: number) => `${n} attempts left — Take test`,
  ctaKecermatanExhausted: "Quota used up — Buy more attempts",
  attemptsPerPurchase: (n: number) => `${n}x attempts per purchase`,
  validUntil: (date: string) => `Attempts valid until ${date}`,
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
