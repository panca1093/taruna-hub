"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  filter: "Filter",
  all: "Semua",
  reset: "Reset",
  cancel: "Batal",
  apply: "Terapkan",
};

const en: typeof id = {
  filter: "Filter",
  all: "All",
  reset: "Reset",
  cancel: "Cancel",
  apply: "Apply",
};

export function useFilterMenuT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
