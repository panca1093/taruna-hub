"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Hasil Tryout",
  loadingResult: "Memuat hasil…",
  irtWarning:
    "Tryout ini menggunakan mode IRT. Beberapa soal masih direspons kurang dari 30 siswa, sehingga skor tetap dihitung dengan bobot kesulitan biasa sampai data cukup untuk kalibrasi IRT.",
  yourTotalScore: "Skor total kamu",
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
  irtWarning:
    "This tryout uses IRT mode. Some questions have been answered by fewer than 30 students, so scores are still calculated using standard difficulty weighting until there's enough data for IRT calibration.",
  yourTotalScore: "Your total score",
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
