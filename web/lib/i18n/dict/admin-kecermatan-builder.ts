"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  editTitle: "Edit Tes Kecermatan",
  createTitle: "Buat Tes Kecermatan",
  lockedDescription: "Sudah ada peserta yang mengerjakan — tes ini tidak dapat diubah lagi",
  saveTest: "Simpan Tes",
  infoHeading: "Info tes",
  titleLabel: "Judul tes",
  programLabel: "Program",
  itemTypeLabel: "Tipe item referensi",
  itemType: {
    char: "Huruf",
    number: "Angka",
    symbol: "Simbol",
    emoji: "Emoji",
    mix: "Campuran",
  },
  descriptionLabel: "Deskripsi",
  coverColorLabel: "Warna sampul",
  coverColorAria: "Pilih warna sampul",
  structureHeading: "Struktur soal",
  itemLengthLabel: "Panjang item (karakter)",
  optionsPerSoalLabel: "Jumlah opsi per soal (ukuran set referensi)",
  soalPerKolomLabel: "Jumlah soal per kolom",
  kolomCountLabel: "Jumlah kolom",
  kolomDurationLabel: "Durasi timer per kolom (menit)",
  structureNote: "Tidak ada mode layar penuh, pencatatan pelanggaran, atau ranking untuk tes kecermatan — berbeda dari tryout.",
  toastIncompleteTitle: "Lengkapi data tes",
  toastIncompleteDescription: "Judul tes wajib diisi.",
  toastUpdated: "Tes kecermatan diperbarui",
  toastCreated: "Tes kecermatan dibuat",
  toastSummary: (title: string, kolom: number, soal: number) => `${title} — ${kolom} kolom, ${soal} soal/kolom`,
};

const en: typeof id = {
  editTitle: "Edit Kecermatan Test",
  createTitle: "Create Kecermatan Test",
  lockedDescription: "Participants have already started — this test can no longer be edited",
  saveTest: "Save Test",
  infoHeading: "Test info",
  titleLabel: "Test title",
  programLabel: "Program",
  itemTypeLabel: "Reference item type",
  itemType: {
    char: "Letter",
    number: "Number",
    symbol: "Symbol",
    emoji: "Emoji",
    mix: "Mixed",
  },
  descriptionLabel: "Description",
  coverColorLabel: "Cover color",
  coverColorAria: "Choose cover color",
  structureHeading: "Question structure",
  itemLengthLabel: "Item length (characters)",
  optionsPerSoalLabel: "Options per question (reference set size)",
  soalPerKolomLabel: "Questions per kolom",
  kolomCountLabel: "Number of kolom",
  kolomDurationLabel: "Timer duration per kolom (minutes)",
  structureNote: "No fullscreen mode, violation logging, or ranking for kecermatan tests — unlike tryouts.",
  toastIncompleteTitle: "Complete test data",
  toastIncompleteDescription: "Test title is required.",
  toastUpdated: "Kecermatan test updated",
  toastCreated: "Kecermatan test created",
  toastSummary: (title: string, kolom: number, soal: number) => `${title} — ${kolom} kolom, ${soal} questions/kolom`,
};

export function useAdminKecermatanBuilderT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
