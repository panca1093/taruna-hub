"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Import Soal Massal",
  description: "Unggah file Excel/CSV atau Word (.docx) sesuai template untuk menambah banyak soal sekaligus.",
  cancel: "Batal",
  confirmImport: "Konfirmasi Import",
  downloadTemplateTitle: "Unduh template",
  downloadTemplateDescription: "Format Excel, CSV, atau Word — kolomnya sama",
  toastTemplateDownloaded: "Template diunduh",
  downloadFailed: "Template gagal diunduh.",
  uploadPrompt: "Klik atau lepas file .xlsx, .csv, atau .docx di sini",
  replaceFile: "Ganti file",
  reading: "Membaca file...",
  importing: "Mengimpor...",
  detected: (valid: number, invalid: number) => `${valid} baris siap diimpor, ${invalid} baris bermasalah`,
  selected: (n: number) => `${n} baris dipilih`,
  columns: {
    row: "Baris",
    status: "Status",
    content: "Soal",
    error: "Kesalahan",
  },
  valid: "Valid",
  invalid: "Bermasalah",
  selectRow: (n: number) => `Pilih baris ${n}`,
  requiredColumns: "Kolom wajib: type, program, subject, difficulty, content — unduh template untuk contohnya",
  uploadFailedTitle: "File tidak bisa dibaca",
  uploadFailedDescription: "Coba lagi atau unduh template terbaru.",
  importedTitle: "Import selesai",
  importedDescription: (created: number, already: number) =>
    `${created} soal dibuat, ${already} soal sebelumnya sudah pernah diimpor.`,
};

const en: typeof id = {
  title: "Bulk Import Questions",
  description: "Upload an Excel/CSV or Word (.docx) file following the template to add many questions at once.",
  cancel: "Cancel",
  confirmImport: "Confirm Import",
  downloadTemplateTitle: "Download template",
  downloadTemplateDescription: "Excel, CSV, or Word format — same columns",
  toastTemplateDownloaded: "Template downloaded",
  downloadFailed: "The template could not be downloaded.",
  uploadPrompt: "Click or drop an .xlsx, .csv, or .docx file here",
  replaceFile: "Change file",
  reading: "Reading the file...",
  importing: "Importing...",
  detected: (valid: number, invalid: number) => `${valid} rows ready to import, ${invalid} rows with problems`,
  selected: (n: number) => `${n} rows selected`,
  columns: {
    row: "Row",
    status: "Status",
    content: "Question",
    error: "Problem",
  },
  valid: "Valid",
  invalid: "Invalid",
  selectRow: (n: number) => `Select row ${n}`,
  requiredColumns: "Required columns: type, program, subject, difficulty, content — download the template for examples",
  uploadFailedTitle: "The file could not be read",
  uploadFailedDescription: "Try again or download the latest template.",
  importedTitle: "Import finished",
  importedDescription: (created: number, already: number) =>
    `${created} questions created, ${already} already imported.`,
};

export function useAdminImportQuestionsModalT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
