"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  catalogCrumb: "Katalog",
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
    mix: "Campuran",
  },
  noProctor:
    "Tidak ada mode layar penuh, pencatatan pelanggaran, atau ranking untuk format ini — hanya latihan kecepatan dan ketelitian yang dinilai per attempt.",
  packageMultiHeading: (n: number) => `${n} tryout dalam paket ini`,
  packageSingleHeading: "Yang akan kamu kerjakan",
  irtModeChip: "Mode IRT",
  subtestStat: (n: number, q: number) => `${n} subtes · ${q} soal`,
  totalMinStat: (min: number) => `${min} menit total`,
  perSubtestDuration: (name: string, min: number) => `${name} · ${min} menit`,
  discountLabel: (pct: number) => `Hemat ${pct}% dari harga normal`,
  ctaKecermatanRemaining: (n: number) => `${n} attempt tersisa — Kerjakan`,
  ctaKecermatanExhausted: "Kuota Habis — Beli Attempt Tambahan",
  ctaBuyNow: "Beli Sekarang",
  ctaAlreadyOwned: "Sudah dimiliki — Kerjakan sekarang",
  attemptsPerPurchase: (n: number) => `${n} kali kesempatan mengerjakan per pembelian`,
  validUntil: (date: string) => `Kesempatan berlaku hingga ${date}`,
  workableUntil: (date: string) => `Dapat dikerjakan hingga ${date}`,
  totalDuration: (min: number) => `Total ${min} menit pengerjaan`,
  repeatable: "Dapat dikerjakan berulang kali selama masa aktif",
};

const en: typeof id = {
  catalogCrumb: "Catalog",
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
    mix: "Mixed",
  },
  noProctor:
    "No fullscreen mode, violation logging, or ranking for this format — only speed and accuracy are scored per attempt.",
  packageMultiHeading: (n: number) => `${n} tryouts in this package`,
  packageSingleHeading: "What you'll take",
  irtModeChip: "IRT Mode",
  subtestStat: (n: number, q: number) => `${n} subtests · ${q} questions`,
  totalMinStat: (min: number) => `${min} min total`,
  perSubtestDuration: (name: string, min: number) => `${name} · ${min} min`,
  discountLabel: (pct: number) => `Save ${pct}% off the normal price`,
  ctaKecermatanRemaining: (n: number) => `${n} attempts left — Take test`,
  ctaKecermatanExhausted: "Quota used up — Buy more attempts",
  ctaBuyNow: "Buy Now",
  ctaAlreadyOwned: "Already owned — Start now",
  attemptsPerPurchase: (n: number) => `${n}x attempts per purchase`,
  validUntil: (date: string) => `Attempts valid until ${date}`,
  workableUntil: (date: string) => `Available until ${date}`,
  totalDuration: (min: number) => `${min} min total to complete`,
  repeatable: "Can be retaken any number of times while active",
};

export function useCatalogDetailT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
