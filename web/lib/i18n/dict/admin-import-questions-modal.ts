"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Import Soal Massal",
  description: "Unggah file Excel/CSV atau Word (.docx) sesuai template untuk menambah banyak soal sekaligus.",
  cancel: "Batal",
  confirmImport: "Konfirmasi Import",
  downloadTemplateTitle: "Unduh template Excel",
  downloadTemplateDescription: "Gunakan format ini agar kolom terbaca dengan benar",
  toastTemplateDownloaded: "Template diunduh",
  uploadPrompt: "Klik untuk unggah file .xlsx, .csv, atau .docx",
  detected: (n: number) => `${n} soal terdeteksi — pratinjau di bawah`,
  columns: {
    subject: "Mapel",
    content: "Soal",
    difficulty: "Kesulitan",
  },
  requiredColumns: "Kolom wajib: program, mapel, tipe, konten, opsi, jawaban_benar, kesulitan, pembahasan",
};

const en: typeof id = {
  title: "Bulk Import Questions",
  description: "Upload an Excel/CSV or Word (.docx) file following the template to add many questions at once.",
  cancel: "Cancel",
  confirmImport: "Confirm Import",
  downloadTemplateTitle: "Download Excel template",
  downloadTemplateDescription: "Use this format so columns are read correctly",
  toastTemplateDownloaded: "Template downloaded",
  uploadPrompt: "Click to upload an .xlsx, .csv, or .docx file",
  detected: (n: number) => `${n} questions detected — preview below`,
  columns: {
    subject: "Subject",
    content: "Question",
    difficulty: "Difficulty",
  },
  requiredColumns: "Required columns: program, subject, type, content, options, correct_answer, difficulty, explanation",
};

export function useAdminImportQuestionsModalT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
