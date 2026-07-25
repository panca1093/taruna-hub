"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Pengaturan",
  deniedMessage: "Hanya Admin yang dapat mengubah konfigurasi sistem.",
  pageDescription: "Konfigurasi sistem tryout",
  saveChanges: "Simpan Perubahan",
  settingsSaved: "Pengaturan disimpan",
  paymentModelTitle: "Model pembayaran",
  paymentModelDescription: "Ketiga metode aktif bersamaan — siswa memilih salah satu saat checkout.",
  paymentGatewayTitle: "Payment Gateway",
  paymentGatewayDescription: "Checkout via Midtrans, akses terbuka otomatis.",
  manualConfirmTitle: "Konfirmasi Manual",
  manualConfirmDescription: "Siswa unggah bukti transfer, admin konfirmasi manual.",
  walletTitle: "Saldo (Wallet)",
  walletDescription: "Bayar dari saldo akun, akses terbuka otomatis.",
  scoringWeightTitle: "Bobot penilaian (weighted scoring)",
  scoringWeightDescription: "Berlaku global untuk semua tryout dalam mode weighted.",
  fieldEasy: "Mudah",
  fieldMedium: "Sedang",
  fieldHard: "Sulit",
  sessionSecurityTitle: "Sesi & keamanan",
  autoLogoutLabel: "Auto-logout saat idle",
  autoLogoutDescription: "Keluarkan sesi otomatis setelah tidak ada aktivitas",
  idleLimitLabel: "Batas idle (menit)",
};

const en: typeof id = {
  pageTitle: "Settings",
  deniedMessage: "Only Admins can change system configuration.",
  pageDescription: "Tryout system configuration",
  saveChanges: "Save Changes",
  settingsSaved: "Settings saved",
  paymentModelTitle: "Payment model",
  paymentModelDescription: "All three methods are active at once — students pick one at checkout.",
  paymentGatewayTitle: "Payment Gateway",
  paymentGatewayDescription: "Checkout via Midtrans, access opens automatically.",
  manualConfirmTitle: "Manual Confirmation",
  manualConfirmDescription: "Student uploads transfer proof, admin confirms manually.",
  walletTitle: "Wallet Balance",
  walletDescription: "Pay from account balance, access opens automatically.",
  scoringWeightTitle: "Scoring weights (weighted scoring)",
  scoringWeightDescription: "Applies globally to all tryouts in weighted mode.",
  fieldEasy: "Easy",
  fieldMedium: "Medium",
  fieldHard: "Hard",
  sessionSecurityTitle: "Session & security",
  autoLogoutLabel: "Auto-logout on idle",
  autoLogoutDescription: "Automatically end the session after no activity",
  idleLimitLabel: "Idle limit (minutes)",
};

export function useSettingsT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
