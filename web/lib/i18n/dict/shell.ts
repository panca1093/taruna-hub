"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  collapse: "Ciutkan",
  expand: "Perluas navigasi",
  openNavigation: "Buka navigasi",
  navigation: "Navigasi akun",
  mainNavigation: "Navigasi utama",
  accountMenu: "Menu akun",
  accountPages: "Halaman akun",
  profile: "Profil",
  orders: "Riwayat pesanan",
  walletTransactions: "Saldo dan transaksi",
  preferences: "Preferensi",
  theme: "Tema",
  themeDark: "Gelap",
  themeLight: "Terang",
  language: "Bahasa",
  languageName: "Indonesia",
  toggleGroup: (name: string) => `Buka atau tutup grup ${name}`,
};

const en: typeof id = {
  collapse: "Collapse",
  expand: "Expand navigation",
  openNavigation: "Open navigation",
  navigation: "Account navigation",
  mainNavigation: "Main navigation",
  accountMenu: "Account menu",
  accountPages: "Account pages",
  profile: "Profile",
  orders: "Order history",
  walletTransactions: "Balance and transactions",
  preferences: "Preferences",
  theme: "Theme",
  themeDark: "Dark",
  themeLight: "Light",
  language: "Language",
  languageName: "English",
  toggleGroup: (name: string) => `Toggle ${name} group`,
};

export function useShellT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
