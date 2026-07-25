"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  catalog: "Katalog",
  howItWorks: "Cara Kerja",
  programs: "Program",
  login: "Masuk",
  register: "Daftar",
};

const en: typeof id = {
  catalog: "Catalog",
  howItWorks: "How It Works",
  programs: "Programs",
  login: "Log In",
  register: "Sign Up",
};

export function usePublicNavT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
