"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
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
  matching: (n: number) => `${n} soal cocok`,
  showAll: "Semua",
  showSelected: (n: number) => `Terpilih · ${n}`,
  selectedCount: (n: number) => `${n} soal terpilih`,
  selectVisible: "Pilih semua",
  clear: "Kosongkan",
  searchPlaceholder: "Cari soal...",
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
  matching: (n: number) => `${n} matching`,
  showAll: "All",
  showSelected: (n: number) => `Selected · ${n}`,
  selectedCount: (n: number) => `${n} selected`,
  selectVisible: "Select all",
  clear: "Clear",
  searchPlaceholder: "Search questions...",
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
