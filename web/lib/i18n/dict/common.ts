"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  cancel: "Batal",
  save: "Simpan",
  close: "Tutup",
  delete: "Hapus",
  edit: "Edit",
  confirm: "Konfirmasi",
  yesDelete: "Ya, Hapus",
  back: "Kembali",
  loading: "Memuat…",
  enableDarkMode: "Aktifkan mode gelap",
  enableLightMode: "Aktifkan mode terang",
  switchToEnglish: "Switch to English",
  switchToIndonesian: "Ganti ke Bahasa Indonesia",
  student: "Siswa",
  operator: "Operator",
  admin: "Admin",
  logout: "Keluar",
  logoutFailed: "Gagal keluar. Coba lagi.",
  selectPlaceholder: "Pilih...",
};

const en: typeof id = {
  cancel: "Cancel",
  save: "Save",
  close: "Close",
  delete: "Delete",
  edit: "Edit",
  confirm: "Confirm",
  yesDelete: "Yes, Delete",
  back: "Back",
  loading: "Loading…",
  enableDarkMode: "Enable dark mode",
  enableLightMode: "Enable light mode",
  switchToEnglish: "Switch to English",
  switchToIndonesian: "Ganti ke Bahasa Indonesia",
  student: "Student",
  operator: "Operator",
  admin: "Admin",
  logout: "Log out",
  logoutFailed: "Could not log out. Try again.",
  selectPlaceholder: "Select...",
};

export function useCommonT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
