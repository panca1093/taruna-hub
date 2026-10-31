"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  loading: "Memuat subtes…",
  loadErrorTitle: "Subtes tidak dapat dimuat",
  loadErrorDescription: "Waktu tetap berjalan di server. Periksa koneksi lalu coba lagi.",
  retry: "Coba Lagi",
  subtestPosition: (current: number, total: number) => `Subtes ${current} / ${total}`,
  timeLeft: "Sisa waktu",
  offlineTitle: "Koneksi terputus",
  offlineBody: (n: number) =>
    n > 0
      ? `${n} jawaban belum tersimpan di server. Waktu tetap berjalan; jawaban dikirim ulang otomatis saat koneksi pulih.`
      : "Jawaban baru belum bisa tersimpan sampai koneksi pulih. Waktu tetap berjalan di server.",
  answerSaved: "Tersimpan di server",
  answerPending: "Belum tersimpan di server",
  pendingCount: (n: number) => `${n} jawaban belum tersimpan`,
  questionProgress: (current: number, total: number) => `Soal ${current} dari ${total}`,
  answerPlaceholder: "Ketik jawabanmu",
  previousButton: "Sebelumnya",
  nextButton: "Selanjutnya",
  finishButton: "Selesaikan subtes",
  navTitle: "Navigasi soal",
  questionNumberLabel: (n: number) => `Soal ${n}`,
  answeredBlank: (answered: number, blank: number) => `${answered} dijawab · ${blank} kosong`,
  submitButton: "Kumpulkan subtes",
  statusRunning: "Berjalan",
  statusSubmitted: "Selesai",
  statusPending: "Belum dimulai",
  backToDetail: "Kembali ke detail",
  backBlocked: (n: number) => `${n} jawaban belum tersimpan, tunggu sampai tersimpan sebelum keluar.`,
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
  confirmHeading: "Kumpulkan subtes ini?",
  confirmBody: (answered: number, blank: number) =>
    `Kamu menjawab ${answered} soal dan membiarkan ${blank} kosong. Setelah dikumpulkan, subtes ini terkunci dan tidak bisa dibuka kembali.`,
  confirmPendingWarning: (n: number) => `${n} jawaban belum tersimpan di server dan tidak akan dihitung.`,
  confirmOfflineWarning: "Koneksi terputus. Pengumpulan akan dicoba saat kamu menekan tombol, dan bisa gagal sampai koneksi pulih.",
  keepWorking: "Kembali ke soal",
  confirmYes: "Ya, kumpulkan",
  submitting: "Mengumpulkan…",
  submitFailed: "Subtes belum terkumpul. Periksa koneksi lalu coba lagi.",
};

const en: typeof id = {
  loading: "Loading subtest…",
  loadErrorTitle: "The subtest could not be loaded",
  loadErrorDescription: "The clock keeps running on the server. Check your connection and try again.",
  retry: "Retry",
  subtestPosition: (current: number, total: number) => `Subtest ${current} / ${total}`,
  timeLeft: "Time left",
  offlineTitle: "Connection lost",
  offlineBody: (n: number) =>
    n > 0
      ? `${n} answers are not saved on the server. The clock keeps running; they are re-sent automatically when the connection returns.`
      : "New answers can't be saved until the connection returns. The clock keeps running on the server.",
  answerSaved: "Saved on the server",
  answerPending: "Not saved on the server yet",
  pendingCount: (n: number) => `${n} answers not saved yet`,
  questionProgress: (current: number, total: number) => `Question ${current} of ${total}`,
  answerPlaceholder: "Type your answer",
  previousButton: "Previous",
  nextButton: "Next",
  finishButton: "Finish subtest",
  navTitle: "Question navigator",
  questionNumberLabel: (n: number) => `Question ${n}`,
  answeredBlank: (answered: number, blank: number) => `${answered} answered · ${blank} blank`,
  submitButton: "Submit subtest",
  statusRunning: "Running",
  statusSubmitted: "Done",
  statusPending: "Not started",
  backToDetail: "Back to detail",
  backBlocked: (n: number) => `${n} answers are not saved yet, wait until they are saved before leaving.`,
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
  confirmHeading: "Submit this subtest?",
  confirmBody: (answered: number, blank: number) =>
    `You answered ${answered} questions and left ${blank} blank. Once submitted, this subtest is locked and can't be reopened.`,
  confirmPendingWarning: (n: number) => `${n} answers are not saved on the server and will not be counted.`,
  confirmOfflineWarning: "You're offline. Submitting will be tried when you press the button and may fail until the connection returns.",
  keepWorking: "Back to questions",
  confirmYes: "Yes, submit",
  submitting: "Submitting…",
  submitFailed: "The subtest was not submitted. Check your connection and try again.",
};

export function useTryoutSubtestT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
