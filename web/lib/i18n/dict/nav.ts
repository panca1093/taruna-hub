"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id: Record<string, string> = {
  "/student": "Dashboard",
  "/catalog": "Katalog",
  "/student/tryouts": "Tryout",
  "/admin": "Ringkasan",
  "/admin/programs": "Program",
  "/admin/questions": "Bank Soal",
  "/admin/tryouts": "Tryout",
  "/admin/kecermatan": "Tes Kecermatan",
  "/admin/item-packages": "Paket Item",
  "/admin/products": "Produk",
  "/admin/orders": "Pesanan",
  "/admin/topups": "Top-up",
  "/admin/students": "Siswa",
  "/admin/results": "Hasil",
  "/admin/kecermatan-results": "Hasil Kecermatan",
  "/admin/monitoring": "Monitoring",
  "/admin/users": "Akun Pengguna",
  "/admin/settings": "Pengaturan",
  "group:overview": "Ringkasan",
  "group:operations": "Operasional",
  "group:content": "Konten",
  "group:finance": "Keuangan",
  "group:system": "Sistem",
};

const en: Record<string, string> = {
  "/student": "Dashboard",
  "/catalog": "Catalog",
  "/student/tryouts": "Tryout",
  "/admin": "Overview",
  "/admin/programs": "Programs",
  "/admin/questions": "Question Bank",
  "/admin/tryouts": "Tryouts",
  "/admin/kecermatan": "Kecermatan Test",
  "/admin/item-packages": "Item Packages",
  "/admin/products": "Products",
  "/admin/orders": "Orders",
  "/admin/topups": "Top-ups",
  "/admin/students": "Students",
  "/admin/results": "Results",
  "/admin/kecermatan-results": "Kecermatan Results",
  "/admin/monitoring": "Monitoring",
  "/admin/users": "User Accounts",
  "/admin/settings": "Settings",
  "group:overview": "Overview",
  "group:operations": "Operations",
  "group:content": "Content",
  "group:finance": "Finance",
  "group:system": "System",
};

export function useNavT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
