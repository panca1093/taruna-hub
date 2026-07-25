"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  roleOperator: "Operator",
  roleAdmin: "Admin",
  accountCreated: (role: string) => `Akun ${role} dibuat`,
  colName: "Nama",
  colRole: "Peran",
  colCreated: "Dibuat",
  pageTitle: "Akun Pengguna",
  pageDescription: "Kelola akun operator dan admin",
  deniedMessage: "Hanya Admin yang dapat mengelola akun operator dan admin.",
  addAccount: "Tambah Akun",
  addStaffTitle: "Tambah Akun Staf",
  cancel: "Batal",
  createAccount: "Buat Akun",
  fieldFullName: "Nama lengkap",
  fieldEmail: "Email",
  fieldRole: "Peran",
  adminInfo: "Admin memiliki akses penuh termasuk manajemen akun dan konfigurasi sistem.",
  operatorInfo: "Operator dapat mengelola konten dan operasional, tanpa akses manajemen akun atau konfigurasi sistem.",
  close: "Tutup",
  deleteTitle: "Hapus akun ini?",
  deleteConfirm: "Ya, Hapus",
  deleteDescription: "Pengguna ini tidak akan bisa masuk lagi setelah dihapus.",
  accountDeleted: "Akun dihapus",
};

const en: typeof id = {
  roleOperator: "Operator",
  roleAdmin: "Admin",
  accountCreated: (role: string) => `${role} account created`,
  colName: "Name",
  colRole: "Role",
  colCreated: "Created",
  pageTitle: "User Accounts",
  pageDescription: "Manage operator and admin accounts",
  deniedMessage: "Only Admins can manage operator and admin accounts.",
  addAccount: "Add Account",
  addStaffTitle: "Add Staff Account",
  cancel: "Cancel",
  createAccount: "Create Account",
  fieldFullName: "Full name",
  fieldEmail: "Email",
  fieldRole: "Role",
  adminInfo: "Admins have full access including account management and system configuration.",
  operatorInfo: "Operators can manage content and operations, without account management or system configuration access.",
  close: "Close",
  deleteTitle: "Delete this account?",
  deleteConfirm: "Yes, Delete",
  deleteDescription: "This user won't be able to sign in again once deleted.",
  accountDeleted: "Account deleted",
};

export function useUsersT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
