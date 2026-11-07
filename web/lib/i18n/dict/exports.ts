"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Ekspor terbaru",
  pendingFile: "Sedang disiapkan",
  statusQueued: "Menunggu",
  statusRunning: "Diproses",
  statusReady: "Siap",
  statusFailed: "Gagal",
  download: "Unduh",
  downloading: "Menyiapkan…",
  failedReason: (code: string) => `Kode kesalahan: ${code}`,
  loadFailed: "Daftar ekspor tidak dapat dimuat.",
  downloadFailed: "Tautan unduhan tidak dapat dibuat.",
};

const en: typeof id = {
  title: "Latest exports",
  pendingFile: "Being prepared",
  statusQueued: "Queued",
  statusRunning: "Running",
  statusReady: "Ready",
  statusFailed: "Failed",
  download: "Download",
  downloading: "Preparing…",
  failedReason: (code: string) => `Error code: ${code}`,
  loadFailed: "The export list could not be loaded.",
  downloadFailed: "The download link could not be created.",
};

export function useExportsT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
