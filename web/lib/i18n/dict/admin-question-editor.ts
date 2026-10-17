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
  contentPlaceholder: "Tulis soal di sini. Gunakan $...$ untuk LaTeX, mis. $x^2 - 5x + 6 = 0$",
  contentHelp: "Konten mendukung format teks (bold/italic/daftar) dan LaTeX — bungkus dengan tanda $ untuk render ekspresi matematika.",
  previewLabel: "Preview",
  explanationLabel: "Pembahasan",
  explanationPlaceholder: "Jelaskan mengapa jawaban tersebut benar",
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
  optionsHintWeighted: "— isi bobot poin tiap opsi",
  optionsHintMark: "— tandai yang benar",
  addOption: "Tambah opsi",
  optionPlaceholder: (label: string) => `Opsi ${label}`,
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
  contentPlaceholder: "Write the question here. Use $...$ for LaTeX, e.g. $x^2 - 5x + 6 = 0$",
  contentHelp: "Content supports text formatting (bold/italic/list) and LaTeX — wrap with $ to render math expressions.",
  previewLabel: "Preview",
  explanationLabel: "Explanation",
  explanationPlaceholder: "Explain why this answer is correct",
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
  optionsHintWeighted: "— fill in the point weight for each option",
  optionsHintMark: "— mark the correct one",
  addOption: "Add option",
  optionPlaceholder: (label: string) => `Option ${label}`,
};

export function useAdminQuestionEditorT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
