"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  bold: "Tebal",
  italic: "Miring",
  bulletList: "Daftar poin",
  numberedList: "Daftar bernomor",
  insertLatex: "Sisipkan LaTeX",
  insertImage: "Sisipkan gambar",
  boldPlaceholder: "teks tebal",
  italicPlaceholder: "teks miring",
  imageAlt: "gambar",
  toastImageTooBig: "Gambar terlalu besar",
  toastImageTooBigDescription: "Ukuran maksimal 5MB.",
};

const en: typeof id = {
  bold: "Bold",
  italic: "Italic",
  bulletList: "Bullet list",
  numberedList: "Numbered list",
  insertLatex: "Insert LaTeX",
  insertImage: "Insert image",
  boldPlaceholder: "bold text",
  italicPlaceholder: "italic text",
  imageAlt: "image",
  toastImageTooBig: "Image too large",
  toastImageTooBigDescription: "Maximum size is 5MB.",
};

export function useAdminRichTextEditorT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
