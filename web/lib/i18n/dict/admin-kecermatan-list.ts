"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Tes Kecermatan",
  description: (n: number) => `${n} tes dibuat`,
  createTest: "Buat Tes",
  emptyTitle: "Belum ada tes kecermatan",
  columns: {
    title: "Tes Kecermatan",
    program: "Program",
    structure: (kolom: number, soal: number) => `${kolom} kolom · ${soal} soal/kolom`,
    duration: (min: number) => `${min} menit`,
    durationHeader: "Timer/kolom",
    structureHeader: "Struktur",
    participants: "Peserta",
  },
  deleteTitle: "Hapus tes kecermatan ini?",
  cancel: "Batal",
  confirmDelete: "Ya, Hapus",
  deleteWarning: "Tindakan ini tidak bisa dibatalkan.",
  toastDeleted: "Tes kecermatan dihapus",
};

const en: typeof id = {
  title: "Kecermatan Tests",
  description: (n: number) => `${n} tests created`,
  createTest: "Create Test",
  emptyTitle: "No kecermatan tests yet",
  columns: {
    title: "Kecermatan Test",
    program: "Program",
    structure: (kolom: number, soal: number) => `${kolom} kolom · ${soal} questions/kolom`,
    duration: (min: number) => `${min} min`,
    durationHeader: "Timer/kolom",
    structureHeader: "Structure",
    participants: "Participants",
  },
  deleteTitle: "Delete this kecermatan test?",
  cancel: "Cancel",
  confirmDelete: "Yes, Delete",
  deleteWarning: "This action cannot be undone.",
  toastDeleted: "Kecermatan test deleted",
};

export function useAdminKecermatanListT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
