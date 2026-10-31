"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  collapse: "Ciutkan",
  expand: "Perluas navigasi",
  openNavigation: "Buka navigasi",
  navigation: "Navigasi akun",
  mainNavigation: "Navigasi utama",
  breadcrumbLabel: "Navigasi halaman",
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
  studentTagline: "Ruang belajarmu",
  staffTagline: "Ruang staf",
  railNote: "Selangkah lebih siap.",
  changeTheme: "Ganti tema",
  profileHint: "Identitas dan keamanan akun",
  ordersHint: "Pembelian dan status pembayaran",
  walletHint: "Saldo, top-up, dan mutasi",
  footer: "Taruna Eduka · Selangkah lebih siap.",
  skipToContent: "Lewati ke konten",
};

const en: typeof id = {
  collapse: "Collapse",
  expand: "Expand navigation",
  openNavigation: "Open navigation",
  navigation: "Account navigation",
  mainNavigation: "Main navigation",
  breadcrumbLabel: "Breadcrumb",
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
  studentTagline: "Your study space",
  staffTagline: "Staff workspace",
  railNote: "One step more ready.",
  changeTheme: "Change theme",
  profileHint: "Identity and account security",
  ordersHint: "Purchases and payment status",
  walletHint: "Balance, top-ups, and ledger",
  footer: "Taruna Eduka · One step more ready.",
  skipToContent: "Skip to content",
};

export function useShellT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
