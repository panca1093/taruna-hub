"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  request: "Unduh PDF",
  preparing: (page: number, total: number) => `Halaman ${page} dari ${total}`,
  preparingStart: "Menyiapkan...",
  toastFailed: "PDF tidak dapat dibuat",
  failedFallback: "Coba lagi sebentar lagi.",
  retry: "Coba lagi",
  toastDone: "PDF siap diunduh",
  subtestMeta: (minutes: number, questions: number) => `${minutes} menit · ${questions} soal`,
  pageOf: (page: number, total: number) => `Halaman ${page} / ${total}`,
  answerLine: "Jawaban:",
  trueLabel: "Benar",
  falseLabel: "Salah",
};

const en: typeof id = {
  request: "Download PDF",
  preparing: (page: number, total: number) => `Page ${page} of ${total}`,
  preparingStart: "Preparing...",
  toastFailed: "The PDF could not be created",
  failedFallback: "Try again in a moment.",
  retry: "Try again",
  toastDone: "The PDF is ready",
  subtestMeta: (minutes: number, questions: number) => `${minutes} minutes · ${questions} questions`,
  pageOf: (page: number, total: number) => `Page ${page} / ${total}`,
  answerLine: "Answer:",
  trueLabel: "True",
  falseLabel: "False",
};

export function useTryoutPdfT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
