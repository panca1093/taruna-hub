"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  submitButton: "Submit Subtes",
  questionProgress: (current: number, total: number) => `Soal ${current} dari ${total}`,
  answerPlaceholder: "Ketik jawabanmu",
  previousButton: "Sebelumnya",
  nextButton: "Selanjutnya",
  answeredCount: (answered: number, total: number) => `Terjawab ${answered}/${total}`,
  screenshotWarningTitle: "Percobaan screenshot terdeteksi",
  screenshotWarningBody:
    "Pelanggaran ini tercatat. Mengambil tangkapan layar soal tidak diperbolehkan selama tryout berlangsung.",
  understood: "Mengerti",
  fullscreenWarningTitle: "Kamu keluar dari mode layar penuh",
  fullscreenWarningBody:
    "Pelanggaran ini tercatat. Timer tetap berjalan — kembali ke mode layar penuh untuk melanjutkan.",
  backToFullscreen: "Kembali ke Layar Penuh",
  confirmSubmitTitle: "Submit subtes ini?",
  confirmSubmitBody: (answered: number, total: number) =>
    `Kamu sudah menjawab ${answered} dari ${total} soal. Setelah disubmit, kamu tidak bisa kembali ke subtes ini.`,
  confirmSubmitYes: "Ya, Submit",
};

const en: typeof id = {
  submitButton: "Submit Subtest",
  questionProgress: (current: number, total: number) => `Question ${current} of ${total}`,
  answerPlaceholder: "Type your answer",
  previousButton: "Previous",
  nextButton: "Next",
  answeredCount: (answered: number, total: number) => `Answered ${answered}/${total}`,
  screenshotWarningTitle: "Screenshot attempt detected",
  screenshotWarningBody:
    "This violation has been logged. Taking screenshots of questions isn't allowed during the tryout.",
  understood: "Got it",
  fullscreenWarningTitle: "You exited fullscreen mode",
  fullscreenWarningBody:
    "This violation has been logged. The timer keeps running — return to fullscreen to continue.",
  backToFullscreen: "Return to Fullscreen",
  confirmSubmitTitle: "Submit this subtest?",
  confirmSubmitBody: (answered: number, total: number) =>
    `You've answered ${answered} of ${total} questions. Once submitted, you can't return to this subtest.`,
  confirmSubmitYes: "Yes, Submit",
};

export function useTryoutSubtestT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
