"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id: Record<string, string> = {
  "/student": "Dashboard",
  "/catalog": "Katalog",
  "/student/tryouts": "Tryout Saya",
  "/student/orders": "Riwayat Pesanan",
  "/student/wallet": "Saldo",
  "/student/profile": "Profil",
  "/admin": "Dashboard",
  "/admin/programs": "Program",
  "/admin/questions": "Bank Soal",
  "/admin/tryouts": "Tryout",
  "/admin/kecermatan": "Tes Kecermatan",
  "/admin/products": "Produk",
  "/admin/orders": "Pesanan",
  "/admin/students": "Siswa",
  "/admin/results": "Hasil",
  "/admin/kecermatan-results": "Hasil Kecermatan",
  "/admin/monitoring": "Monitoring",
  "/admin/users": "Akun Pengguna",
  "/admin/settings": "Pengaturan",
};

const en: Record<string, string> = {
  "/student": "Dashboard",
  "/catalog": "Catalog",
  "/student/tryouts": "My Tryouts",
  "/student/orders": "Order History",
  "/student/wallet": "Balance",
  "/student/profile": "Profile",
  "/admin": "Dashboard",
  "/admin/programs": "Programs",
  "/admin/questions": "Question Bank",
  "/admin/tryouts": "Tryouts",
  "/admin/kecermatan": "Kecermatan Test",
  "/admin/products": "Products",
  "/admin/orders": "Orders",
  "/admin/students": "Students",
  "/admin/results": "Results",
  "/admin/kecermatan-results": "Kecermatan Results",
  "/admin/monitoring": "Monitoring",
  "/admin/users": "User Accounts",
  "/admin/settings": "Settings",
};

export function useNavT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
