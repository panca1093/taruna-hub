"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Profil",
  description: "Kelola informasi akunmu",
  accountInfo: "Informasi akun",
  joinedSince: (date: string) => `Bergabung sejak ${date}`,
  fullName: "Nama lengkap",
  email: "Email",
  programLabel: "Jenjang / program",
  saveChanges: "Simpan Perubahan",
  changePassword: "Ubah kata sandi",
  currentPassword: "Kata sandi saat ini",
  newPassword: "Kata sandi baru",
  updatePassword: "Perbarui Kata Sandi",
  toastProfileUpdated: "Profil diperbarui",
  toastPasswordUpdated: "Kata sandi diperbarui",
};

const en: typeof id = {
  title: "Profile",
  description: "Manage your account information",
  accountInfo: "Account information",
  joinedSince: (date: string) => `Joined since ${date}`,
  fullName: "Full name",
  email: "Email",
  programLabel: "Level / program",
  saveChanges: "Save Changes",
  changePassword: "Change password",
  currentPassword: "Current password",
  newPassword: "New password",
  updatePassword: "Update Password",
  toastProfileUpdated: "Profile updated",
  toastPasswordUpdated: "Password updated",
};

export function useStudentProfileT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
