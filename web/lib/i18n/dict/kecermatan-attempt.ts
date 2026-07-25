"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  eyebrow: "Tes Kecermatan",
  instructionsPart1: (options: number, letter: string) =>
    `Setiap kolom menampilkan ${options} item referensi berlabel A–${letter}.`,
  instructionsPart2: (soal: number) =>
    `Set ini tetap sama untuk seluruh ${soal} soal di kolom itu. Tiap soal menampilkan ulang sebagian item tersebut dengan`,
  instructionsBold: "tepat satu item dihilangkan",
  instructionsPart3: "— pilih label item yang hilang secepat dan seteliti mungkin.",
  kolomTimerBold: (n: number) => `${n} kolom`,
  kolomTimerRest: (min: number) => `· timer ${min} menit per kolom, dibagi rata untuk semua soal di kolom itu.`,
  blankNotWrong: "Soal yang belum terjawab saat timer kolom habis dicatat sebagai kosong, bukan salah.",
  startKolom1: "Mulai Kolom 1",
  kolomFinished: (n: number) => `Kolom ${n} selesai`,
  continueToKolom: (n: number) => `Lanjut ke Kolom ${n}`,
  continueNow: "Lanjut Sekarang",
  kolomProgress: (kolom: number, total: number) => `Kolom ${kolom}/${total}`,
  allAnswered: "Semua soal di kolom ini sudah terjawab.",
  waitingForTimer: "Menunggu waktu kolom berakhir sebelum lanjut ke kolom berikutnya…",
  loadingSoal: "Memuat soal…",
  referenceLabel: "Referensi kolom ini",
  soalProgress: (n: number, total: number) => `Soal ${n} dari ${total}`,
  skip: "Lewati",
  soalPrompt: "Item mana yang hilang dari set referensi di atas?",
  chooseMissingLabel: "Pilih label item yang hilang",
};

const en: typeof id = {
  eyebrow: "Kecermatan Test",
  instructionsPart1: (options: number, letter: string) =>
    `Each kolom shows ${options} labeled reference items A–${letter}.`,
  instructionsPart2: (soal: number) =>
    `This set stays the same for all ${soal} soal in that kolom. Each soal shows a subset of those items with`,
  instructionsBold: "exactly one item removed",
  instructionsPart3: "— pick the label of the missing item as fast and accurately as possible.",
  kolomTimerBold: (n: number) => `${n} kolom`,
  kolomTimerRest: (min: number) => `· ${min} minute timer per kolom, split evenly across all soal in it.`,
  blankNotWrong: "Soal left unanswered when the kolom timer ends are recorded as blank, not wrong.",
  startKolom1: "Start Kolom 1",
  kolomFinished: (n: number) => `Kolom ${n} finished`,
  continueToKolom: (n: number) => `Moving to Kolom ${n}`,
  continueNow: "Continue Now",
  kolomProgress: (kolom: number, total: number) => `Kolom ${kolom}/${total}`,
  allAnswered: "All soal in this kolom have been answered.",
  waitingForTimer: "Waiting for the kolom timer to end before moving to the next kolom…",
  loadingSoal: "Loading soal…",
  referenceLabel: "This kolom's reference",
  soalProgress: (n: number, total: number) => `Soal ${n} of ${total}`,
  skip: "Skip",
  soalPrompt: "Which item is missing from the reference set above?",
  chooseMissingLabel: "Choose the missing item's label",
};

export function useKecermatanAttemptT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
