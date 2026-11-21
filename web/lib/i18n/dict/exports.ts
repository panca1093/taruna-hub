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
  failedMessages: {
    render_failed: "Dokumen gagal dibuat.",
    render_timeout: "Pembuatan dokumen melewati batas waktu.",
    worker_lost: "Proses terhenti sebelum selesai. Coba lagi.",
    too_many_rows: "Data terlalu banyak untuk satu ekspor. Persempit filter.",
    no_renderer: "Pembuat dokumen belum siap. Coba lagi nanti.",
    upload_failed: "Berkas gagal disimpan.",
    storage_unavailable: "Penyimpanan tidak tersedia.",
    internal_error: "Terjadi kesalahan internal.",
  } as Record<string, string>,
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
  failedMessages: {
    render_failed: "The file could not be generated.",
    render_timeout: "Generating the file timed out.",
    worker_lost: "The job stopped before finishing. Try again.",
    too_many_rows: "Too many rows for one export. Narrow the filter.",
    no_renderer: "The exporter is not ready yet. Try again later.",
    upload_failed: "The file could not be stored.",
    storage_unavailable: "Storage is unavailable.",
    internal_error: "An internal error occurred.",
  } as Record<string, string>,
  loadFailed: "The export list could not be loaded.",
  downloadFailed: "The download link could not be created.",
};

export function useExportsT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
