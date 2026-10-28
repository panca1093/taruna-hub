"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Tes Kecermatan",
  description: (n: number) => `${n} tes dibuat`,
  createTest: "Buat Tes",
  emptyTitle: "Belum ada tes kecermatan",
  emptyDescription: "Tes yang Anda buat akan muncul di sini.",
  loading: "Memuat tes kecermatan...",
  loadErrorTitle: "Tes kecermatan gagal dimuat",
  loadErrorDescription: "Periksa koneksi Anda lalu coba lagi.",
  retry: "Coba Lagi",
  loadMore: "Muat Lebih Banyak",
  loadingMore: "Memuat...",
  lockedLabel: "Terkunci",
  lockedHint: "Sudah dikerjakan siswa",
  columns: {
    title: "Tes Kecermatan",
    program: "Program",
    structure: (kolom: number, soal: number) => `${kolom} kolom · ${soal} soal/kolom`,
    duration: (min: number) => `${min} menit`,
    durationHeader: "Timer/kolom",
    structureHeader: "Struktur",
  },
  deleteTitle: "Hapus tes kecermatan ini?",
  cancel: "Batal",
  confirmDelete: "Ya, Hapus",
  deleteWarning: "Tindakan ini tidak bisa dibatalkan.",
  toastDeleted: "Tes kecermatan dihapus",
  toastDeleteFailedTitle: "Tes kecermatan gagal dihapus",
  toastDeleteFailedDescription: "Periksa koneksi Anda lalu coba lagi.",
};

const en: typeof id = {
  title: "Kecermatan Tests",
  description: (n: number) => `${n} tests created`,
  createTest: "Create Test",
  emptyTitle: "No kecermatan tests yet",
  emptyDescription: "Tests you create will appear here.",
  loading: "Loading kecermatan tests...",
  loadErrorTitle: "Kecermatan tests failed to load",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Try Again",
  loadMore: "Load More",
  loadingMore: "Loading...",
  lockedLabel: "Locked",
  lockedHint: "Students have already started",
  columns: {
    title: "Kecermatan Test",
    program: "Program",
    structure: (kolom: number, soal: number) => `${kolom} kolom · ${soal} questions/kolom`,
    duration: (min: number) => `${min} min`,
    durationHeader: "Timer/kolom",
    structureHeader: "Structure",
  },
  deleteTitle: "Delete this kecermatan test?",
  cancel: "Cancel",
  confirmDelete: "Yes, Delete",
  deleteWarning: "This action cannot be undone.",
  toastDeleted: "Kecermatan test deleted",
  toastDeleteFailedTitle: "Kecermatan test failed to delete",
  toastDeleteFailedDescription: "Check your connection and try again.",
};

export function useAdminKecermatanListT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
