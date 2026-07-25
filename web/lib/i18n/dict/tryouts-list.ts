"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Tryout Saya",
  pageDescription: "Tryout dan tes kecermatan yang kamu miliki",
  emptyTitle: "Belum ada tryout yang aktif",
  emptyDescription: "Beli paket tryout dari katalog untuk mulai berlatih.",
  emptyCta: "Lihat Katalog",
  tryoutSectionTitle: "Tryout",
  stateCompleted: "Selesai",
  stateInProgress: "Sedang berjalan",
  stateNotStarted: "Belum dikerjakan",
  ctaViewResult: "Lihat Hasil",
  ctaContinue: "Lanjutkan",
  ctaStart: "Mulai Tryout",
  subtestCount: (n: number) => `${n} subtes`,
  activeUntil: (date: string) => `s.d. ${date}`,
  kecermatanSectionTitle: "Tes Kecermatan",
  attemptsRemaining: (n: number) => `${n} attempt tersisa`,
  quotaExhausted: "Kuota habis",
  kolomSoalStat: (kolom: number, soal: number) => `${kolom} kolom · ${soal} soal/kolom`,
  ctaWork: "Kerjakan",
  ctaViewHistory: "Lihat Riwayat",
};

const en: typeof id = {
  pageTitle: "My Tryouts",
  pageDescription: "Tryouts and kecermatan tests you own",
  emptyTitle: "No active tryouts yet",
  emptyDescription: "Buy a tryout package from the catalog to start practicing.",
  emptyCta: "View Catalog",
  tryoutSectionTitle: "Tryout",
  stateCompleted: "Completed",
  stateInProgress: "In progress",
  stateNotStarted: "Not started",
  ctaViewResult: "View Result",
  ctaContinue: "Continue",
  ctaStart: "Start Tryout",
  subtestCount: (n: number) => `${n} subtests`,
  activeUntil: (date: string) => `until ${date}`,
  kecermatanSectionTitle: "Kecermatan Test",
  attemptsRemaining: (n: number) => `${n} attempts left`,
  quotaExhausted: "Quota used up",
  kolomSoalStat: (kolom: number, soal: number) => `${kolom} kolom · ${soal} soal/kolom`,
  ctaWork: "Take test",
  ctaViewHistory: "View History",
};

export function useTryoutsListT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
