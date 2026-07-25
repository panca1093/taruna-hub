"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  filterAllProgram: "Semua program",
  filterAllSubject: "Semua mapel",
  filterAllType: "Semua tipe",
  filterAllDifficulty: "Semua kesulitan",
  questionType: {
    pg: "Pilihan Ganda",
    pg_multi: "PG Kompleks",
    isian: "Isian Singkat",
    benar_salah: "Benar/Salah",
  },
  difficulty: {
    easy: "Mudah",
    medium: "Sedang",
    hard: "Sulit",
  },
  heading: "Pilih Soal",
  subtitle: (selected: number, filtered: number, pool: number) => `${selected} soal terpilih · ${filtered} dari ${pool} soal ditampilkan`,
  selectVisible: "Pilih yang tampil",
  clear: "Kosongkan",
  searchPlaceholder: "Cari soal atau mapel...",
  onlySelected: "Hanya terpilih",
  weightLabel: (n: number) => `bobot ${n}`,
  emptyText: "Tidak ada soal yang cocok dengan filter ini.",
  cancel: "Batal",
  confirm: (n: number) => `Simpan ${n} Soal`,
};

const en: typeof id = {
  filterAllProgram: "All programs",
  filterAllSubject: "All subjects",
  filterAllType: "All types",
  filterAllDifficulty: "All difficulty",
  questionType: {
    pg: "Multiple Choice",
    pg_multi: "Complex MC",
    isian: "Short Answer",
    benar_salah: "True/False",
  },
  difficulty: {
    easy: "Easy",
    medium: "Medium",
    hard: "Hard",
  },
  heading: "Choose Questions",
  subtitle: (selected: number, filtered: number, pool: number) => `${selected} selected · showing ${filtered} of ${pool} questions`,
  selectVisible: "Select visible",
  clear: "Clear",
  searchPlaceholder: "Search question or subject...",
  onlySelected: "Selected only",
  weightLabel: (n: number) => `weight ${n}`,
  emptyText: "No questions match this filter.",
  cancel: "Cancel",
  confirm: (n: number) => `Save ${n} Questions`,
};

export function useAdminQuestionPickerDrawerT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
