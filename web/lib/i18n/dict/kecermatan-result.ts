"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Hasil Tes Kecermatan",
  loadingResult: "Memuat hasil…",
  loadErrorTitle: "Hasil gagal dimuat",
  loadErrorDescription: "Periksa koneksi lalu coba lagi.",
  retry: "Coba lagi",
  outOf100: "/ 100",
  nilaiAkhirLabel: (n: number) => `Nilai Akhir — Attempt ${n}`,
  scoreSummary: (benar: number, salah: number, kosong: number) => `${benar} benar · ${salah} salah · ${kosong} kosong`,
  scoreKecepatan: "Kecepatan",
  scoreKetahanan: "Ketahanan",
  scoreKetelitian: "Ketelitian",
  compositeScoreTitle: "Skor komposit",
  perKolomTitle: "Rincian per kolom",
  kolomLabel: (n: number) => `Kolom ${n}`,
  benarSalahKosong: "Benar · Salah · Kosong",
  pembahasanTitle: "Pembahasan",
  correctOfTotal: (benar: number, total: number) => `${benar}/${total} benar`,
  soalIndex: (n: number) => `Soal ${n}. `,
  missingItemLabel: "Item yang hilang: ",
  yourAnswer: (label: string | null) => `Jawabanmu: ${label ?? "(kosong)"}`,
};

const en: typeof id = {
  pageTitle: "Kecermatan Test Result",
  loadingResult: "Loading result…",
  loadErrorTitle: "The result could not be loaded",
  loadErrorDescription: "Check the connection and try again.",
  retry: "Try again",
  outOf100: "/ 100",
  nilaiAkhirLabel: (n: number) => `Final Score — Attempt ${n}`,
  scoreSummary: (benar: number, salah: number, kosong: number) => `${benar} correct · ${salah} wrong · ${kosong} blank`,
  scoreKecepatan: "Speed",
  scoreKetahanan: "Endurance",
  scoreKetelitian: "Accuracy",
  compositeScoreTitle: "Composite scores",
  perKolomTitle: "Breakdown per kolom",
  kolomLabel: (n: number) => `Kolom ${n}`,
  benarSalahKosong: "Correct · Wrong · Blank",
  pembahasanTitle: "Answer Review",
  correctOfTotal: (benar: number, total: number) => `${benar}/${total} correct`,
  soalIndex: (n: number) => `Soal ${n}. `,
  missingItemLabel: "Missing item: ",
  yourAnswer: (label: string | null) => `Your answer: ${label ?? "(blank)"}`,
};

export function useKecermatanResultT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
