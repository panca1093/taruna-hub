"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  bold: "Tebal",
  italic: "Miring",
  bulletList: "Daftar poin",
  numberedList: "Daftar bernomor",
  insertLatex: "Sisipkan LaTeX",
  boldPlaceholder: "teks tebal",
  italicPlaceholder: "teks miring",
};

const en: typeof id = {
  bold: "Bold",
  italic: "Italic",
  bulletList: "Bullet list",
  numberedList: "Numbered list",
  insertLatex: "Insert LaTeX",
  boldPlaceholder: "bold text",
  italicPlaceholder: "italic text",
};

export function useAdminRichTextEditorT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
