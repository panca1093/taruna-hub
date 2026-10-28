"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Hasil Tryout",
  loadingResult: "Memuat hasil…",
  yourTotalScore: "Skor total kamu",
  rankPending: "Peringkat belum tersedia",
  explanationsPending: "Pembahasan soal belum tersedia. Untuk saat ini hanya skor yang ditampilkan.",
  loadingReview: "Memuat pembahasan…",
  reviewErrorTitle: "Pembahasan tidak dapat dimuat",
  loadErrorTitle: "Hasil tidak dapat dimuat",
  loadErrorDescription: "Periksa koneksi kamu lalu coba lagi.",
  retry: "Coba Lagi",
  rankLabel: (rank: number, participantCount: number) => `Peringkat ${rank} dari ${participantCount} peserta`,
  subtestScoresTitle: "Skor per subtes",
  correctCount: (correct: number, total: number) => `${correct}/${total} benar`,
  attemptHistoryTitle: "Riwayat percobaan",
  viewAllAttempts: "Lihat semua percobaan",
  explanationWithheldPrefix: "Pembahasan soal untuk tryout ini ditahan oleh admin dan akan tampil setelah",
  explanationWithheldSuffix: "Untuk saat ini hanya skor yang ditampilkan.",
  explanationsTitle: "Pembahasan soal",
  yourAnswerPrefix: "Jawabanmu:",
  emptyAnswer: "(kosong)",
  pointsSuffix: (n: number) => `(${n} poin)`,
  correctAnswerPrefix: "Kunci:",
};

const en: typeof id = {
  pageTitle: "Tryout Result",
  loadingResult: "Loading result…",
  yourTotalScore: "Your total score",
  rankPending: "Rank not available yet",
  explanationsPending: "Explanations are not available yet. For now, only the score is shown.",
  loadingReview: "Loading explanations…",
  reviewErrorTitle: "The explanations could not be loaded",
  loadErrorTitle: "The result could not be loaded",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Retry",
  rankLabel: (rank: number, participantCount: number) => `Rank ${rank} of ${participantCount} participants`,
  subtestScoresTitle: "Score per subtest",
  correctCount: (correct: number, total: number) => `${correct}/${total} correct`,
  attemptHistoryTitle: "Attempt history",
  viewAllAttempts: "View all attempts",
  explanationWithheldPrefix: "The explanation for this tryout is being held by the admin and will show after",
  explanationWithheldSuffix: "For now, only the score is shown.",
  explanationsTitle: "Explanations",
  yourAnswerPrefix: "Your answer:",
  emptyAnswer: "(empty)",
  pointsSuffix: (n: number) => `(${n} points)`,
  correctAnswerPrefix: "Answer key:",
};

export function useTryoutResultT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
