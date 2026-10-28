"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  minutesAgo: (n: number) => `${n} menit lalu`,
  colStudent: "Siswa",
  colTryout: "Tryout",
  colActiveSubtest: "Subtes aktif",
  colStarted: "Mulai",
  colViolations: "Pelanggaran",
  viewDetail: "Lihat Detail",
  pageTitle: "Monitoring",
  pageDescription: "Siswa yang sedang mengerjakan tryout",
  updatedAt: (time: string) => `Diperbarui ${time}`,
  liveSessionsBanner: (n: number) => `${n} sesi sedang berlangsung — halaman ini menyegarkan otomatis setiap 30 detik`,
  emptyTitle: "Tidak ada sesi aktif",
  emptyDescription: "Belum ada siswa yang sedang mengerjakan tryout.",
  detailTitle: "Detail Sesi",
  detailDescription: (name: string, title: string) => `${name} — ${title}`,
  close: "Tutup",
  startedAt: "Mulai mengerjakan",
  activeSubtest: "Subtes aktif",
  violationHistoryTitle: (n: number) => `Riwayat pelanggaran (${n})`,
  noViolations: "Tidak ada pelanggaran tercatat.",
  violationScreenshot: "Percobaan screenshot",
  violationFullscreenExit: "Keluar dari mode layar penuh",
  loading: "Memuat…",
  loadErrorTitle: "Sesi aktif tidak dapat dimuat",
  loadErrorDescription: "Periksa koneksi kamu lalu coba lagi.",
  retry: "Coba Lagi",
  refreshFailed: "Pembaruan terakhir gagal — menampilkan data sebelumnya.",
  timelineError: "Riwayat pelanggaran tidak dapat dimuat.",
};

const en: typeof id = {
  minutesAgo: (n: number) => `${n} minutes ago`,
  colStudent: "Student",
  colTryout: "Tryout",
  colActiveSubtest: "Active subtest",
  colStarted: "Started",
  colViolations: "Violations",
  viewDetail: "View Detail",
  pageTitle: "Monitoring",
  pageDescription: "Students currently taking a tryout",
  updatedAt: (time: string) => `Updated ${time}`,
  liveSessionsBanner: (n: number) => `${n} sessions in progress — this page refreshes automatically every 30 seconds`,
  emptyTitle: "No active sessions",
  emptyDescription: "No students are currently taking a tryout.",
  detailTitle: "Session Detail",
  detailDescription: (name: string, title: string) => `${name} — ${title}`,
  close: "Close",
  startedAt: "Started at",
  activeSubtest: "Active subtest",
  violationHistoryTitle: (n: number) => `Violation history (${n})`,
  noViolations: "No violations recorded.",
  violationScreenshot: "Screenshot attempt",
  violationFullscreenExit: "Exited fullscreen mode",
  loading: "Loading…",
  loadErrorTitle: "Active sessions could not be loaded",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Retry",
  refreshFailed: "The last refresh failed — showing the previous data.",
  timelineError: "The violation history could not be loaded.",
};

export function useMonitoringT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
