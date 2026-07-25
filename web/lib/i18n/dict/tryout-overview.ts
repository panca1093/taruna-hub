"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  noAccess: "Kamu belum memiliki akses ke tryout ini.",
  pageTitle: "Detail Tryout",
  irtModeChip: "Mode IRT",
  completedBanner: "Tryout ini sudah selesai kamu kerjakan.",
  retakeButton: "Kerjakan Lagi",
  viewResultButton: "Lihat Hasil",
  attemptHistoryTitle: "Riwayat Percobaan",
  attemptLabel: (n: number) => `Percobaan ${n}`,
  scoreLabel: "Skor",
  subtestsTitle: "Subtes",
  subtestMeta: (min: number, soal: number) => `${min} menit · ${soal} soal`,
  completedStatus: "Selesai",
  continueButton: "Lanjutkan",
  startButton: "Mulai",
  lockedStatus: "Terkunci",
  footerNotice:
    "Setelah subtes disubmit atau waktu habis, kamu tidak bisa kembali ke subtes tersebut. Browser akan masuk mode layar penuh saat subtes dimulai — jangan keluar dari mode ini.",
};

const en: typeof id = {
  noAccess: "You don't have access to this tryout.",
  pageTitle: "Tryout Detail",
  irtModeChip: "IRT Mode",
  completedBanner: "You've already completed this tryout.",
  retakeButton: "Retake",
  viewResultButton: "View Result",
  attemptHistoryTitle: "Attempt History",
  attemptLabel: (n: number) => `Attempt ${n}`,
  scoreLabel: "Score",
  subtestsTitle: "Subtests",
  subtestMeta: (min: number, soal: number) => `${min} min · ${soal} questions`,
  completedStatus: "Completed",
  continueButton: "Continue",
  startButton: "Start",
  lockedStatus: "Locked",
  footerNotice:
    "Once a subtest is submitted or time runs out, you can't go back to it. The browser will enter fullscreen mode when the subtest starts — don't exit this mode.",
};

export function useTryoutOverviewT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
