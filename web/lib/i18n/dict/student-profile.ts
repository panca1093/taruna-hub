"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  crumbDashboard: "Dashboard",
  title: "Profil",
  description: "Kelola informasi akunmu",
  accountInfo: "Informasi akun",
  fullName: "Nama lengkap",
  email: "Email",
  emailHint: "Email hanya bisa diubah oleh admin.",
  phone: "Nomor HP",
  phoneHint: "Opsional jika akunmu punya email.",
  saveChanges: "Simpan Perubahan",
  saving: "Menyimpan...",
  profileSaved: "Profil diperbarui.",
  changePassword: "Ubah kata sandi",
  currentPassword: "Kata sandi saat ini",
  newPassword: "Kata sandi baru",
  newPasswordHint: "8–72 karakter. Perangkat lain akan keluar otomatis.",
  updatePassword: "Perbarui Kata Sandi",
  updatingPassword: "Memperbarui...",
  passwordSaved: "Kata sandi diperbarui. Perangkat lain telah dikeluarkan.",
  networkError: "Tidak dapat terhubung ke server. Coba lagi.",
};

const en: typeof id = {
  crumbDashboard: "Dashboard",
  title: "Profile",
  description: "Manage your account information",
  accountInfo: "Account information",
  fullName: "Full name",
  email: "Email",
  emailHint: "Only an admin can change your email.",
  phone: "Phone number",
  phoneHint: "Optional if your account has an email.",
  saveChanges: "Save Changes",
  saving: "Saving...",
  profileSaved: "Profile updated.",
  changePassword: "Change password",
  currentPassword: "Current password",
  newPassword: "New password",
  newPasswordHint: "8–72 characters. Your other devices will be signed out.",
  updatePassword: "Update Password",
  updatingPassword: "Updating...",
  passwordSaved: "Password updated. Your other devices were signed out.",
  networkError: "Could not reach the server. Try again.",
};

export function useStudentProfileT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
