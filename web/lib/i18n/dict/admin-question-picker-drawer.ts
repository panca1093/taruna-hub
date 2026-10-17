"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  filterAllProgram: "Semua program",
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
  subtitle: (selected: number, shown: number, total: number) => `${selected} soal terpilih · ${shown} dari ${total} soal ditampilkan`,
  selectVisible: "Pilih yang tampil",
  clear: "Kosongkan",
  searchPlaceholder: "Cari soal...",
  onlySelected: "Hanya terpilih",
  emptyText: "Tidak ada soal yang cocok dengan filter ini.",
  loading: "Memuat soal...",
  loadingMore: "Memuat...",
  loadMore: "Muat Lebih Banyak",
  loadErrorTitle: "Soal gagal dimuat",
  loadErrorDescription: "Periksa koneksi Anda lalu coba lagi.",
  retry: "Coba Lagi",
  cancel: "Batal",
  confirm: (n: number) => `Simpan ${n} Soal`,
};

const en: typeof id = {
  filterAllProgram: "All programs",
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
  subtitle: (selected: number, shown: number, total: number) => `${selected} selected · showing ${shown} of ${total} questions`,
  selectVisible: "Select visible",
  clear: "Clear",
  searchPlaceholder: "Search questions...",
  onlySelected: "Selected only",
  emptyText: "No questions match this filter.",
  loading: "Loading questions...",
  loadingMore: "Loading...",
  loadMore: "Load More",
  loadErrorTitle: "Questions failed to load",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Try Again",
  cancel: "Cancel",
  confirm: (n: number) => `Save ${n} Questions`,
};

export function useAdminQuestionPickerDrawerT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
