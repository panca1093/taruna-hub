"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  noAccess: "Kamu belum memiliki akses ke tryout ini.",
  backToList: "Kembali ke latihan",
  loading: "Memuat tryout…",
  loadErrorTitle: "Tryout tidak dapat dimuat",
  loadErrorDescription: "Periksa koneksi kamu lalu coba lagi.",
  retry: "Coba Lagi",
  pageTitle: "Detail Tryout",
  crumbDashboard: "Dashboard",
  crumbTryout: "Tryout",
  irtModeChip: "Mode IRT",
  statSubtests: "Subtes",
  statQuestions: "Soal",
  statDuration: "Durasi total",
  durationMinutes: (min: number) => `${min} menit`,
  subtestsTitle: "Susunan subtes",
  subtestMeta: (min: number, soal: number) => `${soal} soal · ${min} menit`,
  completedStatus: "Selesai",
  runningStatus: "Berjalan",
  pendingStatus: "Belum dimulai",
  lockedStatus: "Terkunci",
  rulesNotice:
    "Kamu bebas berpindah soal dalam subtes yang sedang berjalan. Setelah dikumpulkan atau waktu habis, subtes tersebut terkunci.",
  accessTitle: "Akses latihan",
  accessOwned: "Sudah dimiliki",
  accessUntil: (date: string) => `Akses sampai ${date}`,
  ctaStart: "Mulai tryout",
  ctaContinueSubtest: (name: string) => `Lanjutkan ${name}`,
  ctaNextSubtest: (name: string) => `Lanjut ke ${name}`,
  ctaRetake: "Kerjakan lagi",
  ctaBack: "Kembali ke latihan",
  historyTitle: "Riwayat percobaan",
  historyEmpty: "Belum ada percobaan selesai. Hasil pertama akan tersimpan di sini.",
  attemptLabel: (n: number) => `Percobaan ${n}`,
  openAttemptLabel: (n: number) => `Buka hasil percobaan ${n}`,
  inProgressNote: "Percobaan sedang berjalan. Waktu subtes yang berjalan tetap dihitung server.",
};

const en: typeof id = {
  noAccess: "You don't have access to this tryout.",
  backToList: "Back to practice",
  loading: "Loading tryout…",
  loadErrorTitle: "The tryout could not be loaded",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Retry",
  pageTitle: "Tryout Detail",
  crumbDashboard: "Dashboard",
  crumbTryout: "Tryout",
  irtModeChip: "IRT Mode",
  statSubtests: "Subtests",
  statQuestions: "Questions",
  statDuration: "Total duration",
  durationMinutes: (min: number) => `${min} min`,
  subtestsTitle: "Subtest lineup",
  subtestMeta: (min: number, soal: number) => `${soal} questions · ${min} min`,
  completedStatus: "Completed",
  runningStatus: "Running",
  pendingStatus: "Not started",
  lockedStatus: "Locked",
  rulesNotice:
    "You can move freely between questions in the running subtest. Once it is submitted or time runs out, that subtest is locked.",
  accessTitle: "Practice access",
  accessOwned: "You own this",
  accessUntil: (date: string) => `Access until ${date}`,
  ctaStart: "Start tryout",
  ctaContinueSubtest: (name: string) => `Continue ${name}`,
  ctaNextSubtest: (name: string) => `Go to ${name}`,
  ctaRetake: "Retake",
  ctaBack: "Back to practice",
  historyTitle: "Attempt history",
  historyEmpty: "No completed attempts yet. Your first result will be saved here.",
  attemptLabel: (n: number) => `Attempt ${n}`,
  openAttemptLabel: (n: number) => `Open result of attempt ${n}`,
  inProgressNote: "An attempt is in progress. The running subtest's time keeps counting on the server.",
};

export function useTryoutOverviewT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
