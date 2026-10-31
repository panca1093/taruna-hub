"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  collapse: "Ciutkan",
  expand: "Perluas navigasi",
  openNavigation: "Buka navigasi",
  navigation: "Navigasi akun",
};

const en: typeof id = {
  collapse: "Collapse",
  expand: "Expand navigation",
  openNavigation: "Open navigation",
  navigation: "Account navigation",
};

export function useShellT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
