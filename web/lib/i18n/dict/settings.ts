"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Pengaturan",
  deniedMessage: "Hanya Admin yang dapat mengubah konfigurasi sistem.",
  pageDescription: "Konfigurasi sistem tryout",
  paymentModelTitle: "Model pembayaran",
  paymentModelDescription: "Ketiga metode aktif bersamaan — siswa memilih salah satu saat checkout.",
  paymentGatewayTitle: "Payment Gateway",
  paymentGatewayDescription: "Checkout via Midtrans, akses terbuka otomatis.",
  manualConfirmTitle: "Konfirmasi Manual",
  manualConfirmDescription: "Siswa unggah bukti transfer, admin konfirmasi manual.",
  walletTitle: "Saldo (Wallet)",
  walletDescription: "Bayar dari saldo akun, akses terbuka otomatis.",
  scoringWeightTitle: "Bobot penilaian (weighted scoring)",
  scoringWeightDescription: "Berlaku untuk attempt baru pada semua tryout. Attempt yang sudah berjalan memakai bobot saat dimulai.",
  fieldEasy: "Mudah",
  fieldMedium: "Sedang",
  fieldHard: "Sulit",
  save: "Simpan",
  saving: "Menyimpan...",
  loading: "Memuat pengaturan...",
  loadErrorDescription: "Periksa koneksi Anda lalu coba lagi.",
  retry: "Coba Lagi",
  toastSaved: "Bobot penilaian disimpan",
  toastSaveFailed: "Gagal menyimpan",
};

const en: typeof id = {
  pageTitle: "Settings",
  deniedMessage: "Only Admins can change system configuration.",
  pageDescription: "Tryout system configuration",
  paymentModelTitle: "Payment model",
  paymentModelDescription: "All three methods are active at once — students pick one at checkout.",
  paymentGatewayTitle: "Payment Gateway",
  paymentGatewayDescription: "Checkout via Midtrans, access opens automatically.",
  manualConfirmTitle: "Manual Confirmation",
  manualConfirmDescription: "Student uploads transfer proof, admin confirms manually.",
  walletTitle: "Wallet Balance",
  walletDescription: "Pay from account balance, access opens automatically.",
  scoringWeightTitle: "Scoring weights (weighted scoring)",
  scoringWeightDescription: "Applies to new attempts on every tryout. An attempt already started keeps the weights it began with.",
  fieldEasy: "Easy",
  fieldMedium: "Medium",
  fieldHard: "Hard",
  save: "Save",
  saving: "Saving...",
  loading: "Loading settings...",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Retry",
  toastSaved: "Scoring weights saved",
  toastSaveFailed: "Could not save",
};

export function useSettingsT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
