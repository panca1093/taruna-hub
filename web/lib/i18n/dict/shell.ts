"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  collapse: "Ciutkan",
};

const en: typeof id = {
  collapse: "Collapse",
};

export function useShellT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
