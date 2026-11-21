"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  imageLoading: "Memuat gambar",
  imageFailed: "Gambar gagal dimuat",
};

const en: typeof id = {
  imageLoading: "Loading image",
  imageFailed: "Image failed to load",
};

export function useRichDocT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
