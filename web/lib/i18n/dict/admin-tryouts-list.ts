"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Tryout",
  description: (n: number) => `${n} tryout dibuat`,
  createTryout: "Buat Tryout",
  emptyTitle: "Belum ada tryout",
  emptyDescription: "Tryout yang Anda buat akan muncul di sini.",
  loading: "Memuat tryout...",
  loadErrorTitle: "Tryout gagal dimuat",
  loadErrorDescription: "Periksa koneksi Anda lalu coba lagi.",
  retry: "Coba Lagi",
  loadMore: "Muat Lebih Banyak",
  loadingMore: "Memuat...",
  columns: {
    title: "Tryout",
    program: "Program",
    mode: "Penilaian",
    subtests: "Subtes",
    status: "Status",
    activeUntil: "Aktif hingga",
  },
  scoringMode: {
    irt: "IRT",
    weighted: "Weighted",
  },
  lockedLabel: "Terkunci",
  lockedHint: "Sudah dikerjakan siswa",
  deleteTitle: "Hapus tryout ini?",
  deleteDescription: "Tindakan ini tidak bisa dibatalkan.",
  deleteWarning: "Tindakan ini tidak bisa dibatalkan.",
  cancel: "Batal",
  confirmDelete: "Ya, Hapus",
  toastDeleted: "Tryout dihapus",
  toastDeleteFailedTitle: "Tryout gagal dihapus",
  toastDeleteFailedDescription: "Periksa koneksi Anda lalu coba lagi.",
  requireRoleTitle: "Akses ditolak",
  requireRoleDenied: "Halaman tryout hanya untuk Admin dan Operator.",
};

const en: typeof id = {
  title: "Tryouts",
  description: (n: number) => `${n} tryouts created`,
  createTryout: "Create Tryout",
  emptyTitle: "No tryouts yet",
  emptyDescription: "Tryouts you create will appear here.",
  loading: "Loading tryouts...",
  loadErrorTitle: "Tryouts failed to load",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Try Again",
  loadMore: "Load More",
  loadingMore: "Loading...",
  columns: {
    title: "Tryout",
    program: "Program",
    mode: "Scoring",
    subtests: "Subtests",
    status: "Status",
    activeUntil: "Active until",
  },
  scoringMode: {
    irt: "IRT",
    weighted: "Weighted",
  },
  lockedLabel: "Locked",
  lockedHint: "Students have already attempted it",
  deleteTitle: "Delete this tryout?",
  deleteDescription: "This action cannot be undone.",
  deleteWarning: "This action cannot be undone.",
  cancel: "Cancel",
  confirmDelete: "Yes, Delete",
  toastDeleted: "Tryout deleted",
  toastDeleteFailedTitle: "Tryout failed to delete",
  toastDeleteFailedDescription: "Check your connection and try again.",
  requireRoleTitle: "Access denied",
  requireRoleDenied: "The tryout page is for Admin and Operator only.",
};

export function useAdminTryoutsListT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
