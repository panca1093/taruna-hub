"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  submitButton: "Submit Subtes",
  loading: "Memuat subtes…",
  loadErrorTitle: "Subtes tidak dapat dimuat",
  loadErrorDescription: "Waktu tetap berjalan di server. Periksa koneksi lalu coba lagi.",
  retry: "Coba Lagi",
  answerSaved: "Tersimpan",
  answerPending: "Belum tersimpan di server",
  pendingCount: (n: number) => `${n} jawaban belum tersimpan`,
  confirmPendingWarning: (n: number) => `${n} jawaban belum tersimpan di server dan tidak akan dihitung.`,
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
  fullscreenRequestTitle: "Gunakan mode layar penuh",
  fullscreenRequestBody: "Tryout ini berjalan dalam mode layar penuh. Timer tetap berjalan — aktifkan layar penuh untuk melanjutkan.",
  backToFullscreen: "Kembali ke Layar Penuh",
  confirmSubmitTitle: "Submit subtes ini?",
  confirmSubmitBody: (answered: number, total: number) =>
    `Kamu sudah menjawab ${answered} dari ${total} soal. Setelah disubmit, kamu tidak bisa kembali ke subtes ini.`,
  confirmSubmitYes: "Ya, Submit",
};

const en: typeof id = {
  submitButton: "Submit Subtest",
  loading: "Loading subtest…",
  loadErrorTitle: "The subtest could not be loaded",
  loadErrorDescription: "The clock keeps running on the server. Check your connection and try again.",
  retry: "Retry",
  answerSaved: "Saved",
  answerPending: "Not saved on the server yet",
  pendingCount: (n: number) => `${n} answers not saved yet`,
  confirmPendingWarning: (n: number) => `${n} answers are not saved on the server and will not be counted.`,
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
  fullscreenRequestTitle: "Use fullscreen mode",
  fullscreenRequestBody: "This tryout runs in fullscreen. The timer keeps running — turn on fullscreen to continue.",
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
