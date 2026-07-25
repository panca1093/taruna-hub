"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Tryout",
  description: (n: number) => `${n} tryout dibuat`,
  createTryout: "Buat Tryout",
  emptyTitle: "Belum ada tryout",
  columns: {
    title: "Tryout",
    program: "Program",
    mode: "Penilaian",
    subtests: "Subtes",
    participants: "Peserta",
    activeUntil: "Aktif hingga",
  },
  scoringMode: {
    irt: "IRT",
    weighted: "Weighted",
  },
  deleteTitle: "Hapus tryout ini?",
  cancel: "Batal",
  confirmDelete: "Ya, Hapus",
  deleteWarning: "Tindakan ini tidak bisa dibatalkan.",
  toastDeleted: "Tryout dihapus",
};

const en: typeof id = {
  title: "Tryouts",
  description: (n: number) => `${n} tryouts created`,
  createTryout: "Create Tryout",
  emptyTitle: "No tryouts yet",
  columns: {
    title: "Tryout",
    program: "Program",
    mode: "Scoring",
    subtests: "Subtests",
    participants: "Participants",
    activeUntil: "Active until",
  },
  scoringMode: {
    irt: "IRT",
    weighted: "Weighted",
  },
  deleteTitle: "Delete this tryout?",
  cancel: "Cancel",
  confirmDelete: "Yes, Delete",
  deleteWarning: "This action cannot be undone.",
  toastDeleted: "Tryout deleted",
};

export function useAdminTryoutsListT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
