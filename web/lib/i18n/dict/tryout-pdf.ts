"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  request: "Unduh PDF",
  requesting: "Memproses...",
  toastQueued: "PDF sedang disiapkan",
  toastQueuedDescription: "Unduh dari daftar ekspor di bawah setelah statusnya Siap.",
  toastFailed: "PDF tidak dapat diminta",
  failedFallback: "Coba lagi sebentar lagi.",
};

const en: typeof id = {
  request: "Download PDF",
  requesting: "Working...",
  toastQueued: "The PDF is being prepared",
  toastQueuedDescription: "Download it from the export list below once its status is Ready.",
  toastFailed: "The PDF could not be requested",
  failedFallback: "Try again in a moment.",
};

export function useTryoutPdfT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
