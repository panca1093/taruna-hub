"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Pembahasan Tryout",
  crumbDashboard: "Dashboard",
  crumbResult: "Hasil tryout",
  loading: "Memuat pembahasan…",
  loadErrorTitle: "Pembahasan tidak dapat dimuat",
  loadErrorDescription: "Periksa koneksi kamu lalu coba lagi.",
  retry: "Coba Lagi",
  unavailableTitle: "Pembahasan percobaan ini tidak tersedia",
  unavailableDescription: "Percobaan tidak ditemukan atau bukan milik akunmu. Pilih percobaan dari riwayat.",
  notReadyTitle: "Percobaan ini belum selesai",
  notReadyDescription: "Pembahasan tersedia setelah percobaan selesai.",
  toDetail: "Buka detail tryout",
  heldTitle: "Pembahasan belum dibuka",
  heldBody: "Pembahasan ditahan dan belum bisa dilihat. Skor tetap bisa kamu lihat di halaman hasil.",
  heldUntil: (date: string) => `Pembahasan dibuka mulai ${date}.`,
  backToResult: "Kembali ke hasil",
  heading: "Pahami setiap jawaban.",
  headingMeta: (title: string, attempt: number) => `${title} · Percobaan ${attempt}`,
  subtestsLabel: "Subtes",
  correctCount: (correct: number, total: number) => `${correct}/${total} benar`,
  statusCorrect: "Benar",
  statusWrong: "Salah",
  statusBlank: "Kosong",
  yourAnswerPrefix: "Jawabanmu:",
  correctAnswerPrefix: "Jawaban benar:",
  emptyAnswer: "(kosong)",
  pointsSuffix: (n: number) => `(${n} poin)`,
  noExplanation: "Belum ada pembahasan untuk soal ini.",
};

const en: typeof id = {
  pageTitle: "Tryout Explanations",
  crumbDashboard: "Dashboard",
  crumbResult: "Tryout result",
  loading: "Loading explanations…",
  loadErrorTitle: "The explanations could not be loaded",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Retry",
  unavailableTitle: "This attempt's explanations aren't available",
  unavailableDescription: "The attempt wasn't found or doesn't belong to your account. Pick an attempt from the history.",
  notReadyTitle: "This attempt isn't finished yet",
  notReadyDescription: "Explanations are available once the attempt is complete.",
  toDetail: "Open tryout detail",
  heldTitle: "Explanations are not open yet",
  heldBody: "The explanations are being held and can't be viewed yet. You can still see your score on the result page.",
  heldUntil: (date: string) => `Explanations open from ${date}.`,
  backToResult: "Back to result",
  heading: "Understand every answer.",
  headingMeta: (title: string, attempt: number) => `${title} · Attempt ${attempt}`,
  subtestsLabel: "Subtests",
  correctCount: (correct: number, total: number) => `${correct}/${total} correct`,
  statusCorrect: "Correct",
  statusWrong: "Wrong",
  statusBlank: "Blank",
  yourAnswerPrefix: "Your answer:",
  correctAnswerPrefix: "Correct answer:",
  emptyAnswer: "(empty)",
  pointsSuffix: (n: number) => `(${n} points)`,
  noExplanation: "No explanation for this question yet.",
};

export function useTryoutReviewT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
