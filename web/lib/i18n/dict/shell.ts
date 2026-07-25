"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  collapse: "Ciutkan",
  viewAsDemo: "Lihat sebagai (demo)",
};

const en: typeof id = {
  collapse: "Collapse",
  viewAsDemo: "View as (demo)",
};

export function useShellT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
