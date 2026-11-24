"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  program: "Program",
  subject: "Mata pelajaran",
  chapter: "Bab / materi",
  type: "Tipe soal",
  difficulty: "Kesulitan",
  all: "Semua",
  filter: "Filter",
  allPrograms: "Semua program",
  allSubjects: "Semua mata pelajaran",
  allChapters: "Semua bab",
  chapterNeedsSubject: "Pilih mata pelajaran dulu",
  reset: "Reset",
  cancel: "Batal",
  apply: "Terapkan",
  questionType: {
    pg: "Pilihan Ganda",
    pg_multi: "PG Kompleks",
    isian: "Isian Singkat",
    benar_salah: "Benar/Salah",
  },
  difficultyLevel: {
    easy: "Mudah",
    medium: "Sedang",
    hard: "Sulit",
  },
};

const en: typeof id = {
  program: "Program",
  subject: "Subject",
  chapter: "Chapter",
  type: "Type",
  difficulty: "Difficulty",
  all: "All",
  filter: "Filter",
  allPrograms: "All programs",
  allSubjects: "All subjects",
  allChapters: "All chapters",
  chapterNeedsSubject: "Pick a subject first",
  reset: "Reset",
  cancel: "Cancel",
  apply: "Apply",
  questionType: {
    pg: "Multiple Choice",
    pg_multi: "Complex MC",
    isian: "Short Answer",
    benar_salah: "True/False",
  },
  difficultyLevel: {
    easy: "Easy",
    medium: "Medium",
    hard: "Hard",
  },
};

export function useAdminQuestionFiltersT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
