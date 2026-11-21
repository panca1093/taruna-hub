"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  typeOptions: {
    pg: "Pilihan Ganda",
    pg_multi: "PG Kompleks (jawaban > 1)",
    isian: "Isian Singkat",
    benar_salah: "Benar / Salah",
  },
  difficultyOptions: {
    easy: "Mudah",
    medium: "Sedang",
    hard: "Sulit",
  },
  contentLabel: "Konten soal",
  contentPlaceholder: "Tulis soal di sini",
  explanationLabel: "Pembahasan",
  explanationHint: "Opsional, tampil setelah tryout selesai",
  explanationPlaceholder: "Jelaskan mengapa jawaban tersebut benar",
  classificationTitle: "Klasifikasi",
  answerTitle: "Jawaban",
  programLabel: "Program",
  subjectLabel: "Mata pelajaran",
  subjectPlaceholderEmpty: "Program ini belum punya mapel",
  subjectPlaceholder: "Pilih mapel",
  chapterLabel: "Bab materi",
  chapterPlaceholderEmpty: "Mapel ini belum punya bab materi",
  chapterPlaceholder: "Pilih bab materi (opsional)",
  typeLabel: "Tipe soal",
  difficultyLabel: "Tingkat kesulitan",
  optionWeightedLabel: "Bobot per opsi jawaban",
  optionWeightedDescription:
    "Semua opsi dianggap benar; siswa mendapat poin sesuai bobot opsi yang dipilih. Opsi tanpa bobot otomatis bernilai 1. Cocok untuk soal TKP/psikotes.",
  correctAnswerLabel: "Jawaban benar",
  correctAnswerDescription: "Satu jawaban per baris. Pencocokan tidak membedakan huruf besar-kecil.",
  caseSensitiveLabel: "Bedakan huruf besar-kecil",
  caseSensitiveDescription: "Jawaban hanya benar bila huruf besar-kecilnya sama persis.",
  optionsLabel: "Opsi jawaban",
  optionsHintWeighted: "Isi bobot di kiri tiap opsi",
  optionsHintMark: "Klik huruf untuk menandai jawaban benar",
  addOption: "Tambah opsi",
  optionPlaceholder: "Tulis opsi jawaban",
  optionField: (letter: string) => `Opsi ${letter}`,
  markCorrect: (letter: string) => `Tandai opsi ${letter} sebagai jawaban benar`,
  optionPoints: (letter: string) => `Bobot opsi ${letter}`,
  removeOption: (letter: string) => `Hapus opsi ${letter}`,
};

const en: typeof id = {
  typeOptions: {
    pg: "Multiple Choice",
    pg_multi: "Complex MC (answer > 1)",
    isian: "Short Answer",
    benar_salah: "True / False",
  },
  difficultyOptions: {
    easy: "Easy",
    medium: "Medium",
    hard: "Hard",
  },
  contentLabel: "Question content",
  contentPlaceholder: "Write the question here",
  explanationLabel: "Explanation",
  explanationHint: "Optional, shown after the tryout ends",
  explanationPlaceholder: "Explain why this answer is correct",
  classificationTitle: "Classification",
  answerTitle: "Answer",
  programLabel: "Program",
  subjectLabel: "Subject",
  subjectPlaceholderEmpty: "This program has no subjects yet",
  subjectPlaceholder: "Choose subject",
  chapterLabel: "Chapter",
  chapterPlaceholderEmpty: "This subject has no chapters yet",
  chapterPlaceholder: "Choose chapter (optional)",
  typeLabel: "Question type",
  difficultyLabel: "Difficulty level",
  optionWeightedLabel: "Weight per answer option",
  optionWeightedDescription:
    "All options are considered correct; students get points based on the weight of the option they chose. Options without a weight default to 1. Good for personality/psychometric questions.",
  correctAnswerLabel: "Correct answer",
  correctAnswerDescription: "One answer per line. Matching does not distinguish upper and lower case.",
  caseSensitiveLabel: "Case-sensitive",
  caseSensitiveDescription: "An answer is only correct when its capitalisation matches exactly.",
  optionsLabel: "Answer options",
  optionsHintWeighted: "Fill in the weight on the left of each option",
  optionsHintMark: "Click the letter to mark the correct answer",
  addOption: "Add option",
  optionPlaceholder: "Write the answer option",
  optionField: (letter: string) => `Option ${letter}`,
  markCorrect: (letter: string) => `Mark option ${letter} as the correct answer`,
  optionPoints: (letter: string) => `Weight of option ${letter}`,
  removeOption: (letter: string) => `Delete option ${letter}`,
};

export function useAdminQuestionEditorT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
