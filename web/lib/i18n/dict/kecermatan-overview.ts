"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Detail Tes Kecermatan",
  loading: "Memuat tes…",
  loadErrorTitle: "Tes gagal dimuat",
  loadErrorDescription: "Periksa koneksi lalu coba lagi.",
  retry: "Coba lagi",
  noAccess: "Kamu belum memiliki akses ke tes kecermatan ini.",
  chipKecermatan: "Tes Kecermatan",
  attemptsRemaining: (n: number) => `${n} attempt tersisa`,
  kolomStat: (kolom: number, soal: number, min: number) => `${kolom} kolom · ${soal} soal/kolom · ${min} menit/kolom`,
  ctaContinue: "Lanjutkan",
  ctaStartNew: "Mulai Attempt Baru",
  ctaBuyMore: "Beli Attempt Tambahan",
  historyTitle: "Riwayat Attempt",
  attemptLabel: (n: number) => `Attempt ${n}`,
  nilaiAkhir: "Nilai Akhir",
  noHistoryInfo:
    "Setiap kolom memiliki set referensi berlabel yang tetap sama untuk semua soal di kolom itu. Tugasmu: temukan label item yang hilang pada tiap soal, secepat dan seteliti mungkin sebelum timer kolom habis.",
};

const en: typeof id = {
  pageTitle: "Kecermatan Test Detail",
  loading: "Loading test…",
  loadErrorTitle: "The test could not be loaded",
  loadErrorDescription: "Check the connection and try again.",
  retry: "Try again",
  noAccess: "You don't have access to this kecermatan test yet.",
  chipKecermatan: "Kecermatan Test",
  attemptsRemaining: (n: number) => `${n} attempts left`,
  kolomStat: (kolom: number, soal: number, min: number) => `${kolom} kolom · ${soal} soal/kolom · ${min} min/kolom`,
  ctaContinue: "Continue",
  ctaStartNew: "Start New Attempt",
  ctaBuyMore: "Buy More Attempts",
  historyTitle: "Attempt History",
  attemptLabel: (n: number) => `Attempt ${n}`,
  nilaiAkhir: "Final Score",
  noHistoryInfo:
    "Each kolom has a labeled reference set that stays the same for every soal in it. Your task: find the missing item's label on each soal, as fast and accurately as possible before the kolom timer runs out.",
};

export function useKecermatanOverviewT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
