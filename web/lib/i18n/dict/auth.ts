"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  // shared
  emailOrPhoneLabel: "Email atau No. HP",
  passwordLabel: "Kata sandi",
  passwordPlaceholder: "••••••••",
  emailLabel: "Email",
  emailPlaceholder: "nama@email.com",
  prototypeNote: 'Prototipe — isi apa saja lalu tekan "Masuk" untuk melihat tampilan.',
  backToLogin: "Kembali ke halaman masuk",

  // login (student)
  loginTitle: "Masuk",
  loginSubtitle: "Masuk sebagai siswa untuk mengerjakan tryout.",
  loginEmailOrPhonePlaceholder: "nama@email.com atau 08xxxxxxxxxx",
  forgotPasswordLink: "Lupa kata sandi?",
  submitStudent: "Masuk sebagai siswa",
  noAccount: "Belum punya akun?",
  registerLink: "Daftar sebagai siswa",
  staffPrompt: "Operator atau admin?",
  staffPortalLink: "Masuk ke portal internal",

  // login/staff
  staffTitle: "Portal Internal",
  staffSubtitle: "Masuk sebagai operator atau admin untuk mengelola operasional.",
  roleOperator: "Operator",
  roleAdmin: "Admin",
  staffEmailOrPhonePlaceholder: "nama@tarunaeduka.id atau 08xxxxxxxxxx",
  submitStaff: (role: string) => `Masuk sebagai ${role}`,
  studentPrompt: "Siswa?",
  studentLoginLink: "Masuk ke akun siswa",

  // register
  registerTitle: "Daftar sebagai siswa",
  registerSubtitle: "Registrasi operator dan admin dibuat oleh admin dari dashboard, bukan di sini.",
  fullNameLabel: "Nama lengkap",
  fullNamePlaceholder: "Nama kamu",
  phoneLabel: "No. HP",
  phonePlaceholder: "08xxxxxxxxxx",
  registerPasswordPlaceholder: "Minimal 8 karakter",
  programLabel: "Jenjang / program",
  programPlaceholder: "Pilih jenjang",
  submitRegister: "Buat Akun",
  haveAccount: "Sudah punya akun?",
  loginLink: "Masuk",
  toastAccountCreatedTitle: "Akun berhasil dibuat",
  toastAccountCreatedDescription: "Selamat datang! Yuk cari tryout pertamamu.",

  // forgot-password
  forgotTitle: "Lupa kata sandi",
  forgotSubtitle: "Masukkan email akunmu — kami akan kirim tautan untuk mengatur ulang kata sandi.",
  sentTitle: "Cek email kamu",
  sentDescriptionBefore: "Jika",
  sentDescriptionAfter: "terdaftar, kami sudah mengirim tautan untuk mengatur ulang kata sandi.",
  submitReset: "Kirim Tautan Reset",
};

const en: typeof id = {
  // shared
  emailOrPhoneLabel: "Email or Phone Number",
  passwordLabel: "Password",
  passwordPlaceholder: "••••••••",
  emailLabel: "Email",
  emailPlaceholder: "name@email.com",
  prototypeNote: 'Prototype — fill in anything and press "Log In" to see the preview.',
  backToLogin: "Back to login page",

  // login (student)
  loginTitle: "Log In",
  loginSubtitle: "Log in as a student to take tryouts.",
  loginEmailOrPhonePlaceholder: "name@email.com or 08xxxxxxxxxx",
  forgotPasswordLink: "Forgot password?",
  submitStudent: "Log in as a student",
  noAccount: "Don't have an account?",
  registerLink: "Sign up as a student",
  staffPrompt: "Operator or admin?",
  staffPortalLink: "Go to internal portal",

  // login/staff
  staffTitle: "Internal Portal",
  staffSubtitle: "Log in as an operator or admin to manage operations.",
  roleOperator: "Operator",
  roleAdmin: "Admin",
  staffEmailOrPhonePlaceholder: "name@tarunaeduka.id or 08xxxxxxxxxx",
  submitStaff: (role: string) => `Log in as ${role}`,
  studentPrompt: "Student?",
  studentLoginLink: "Log in to student account",

  // register
  registerTitle: "Sign up as a student",
  registerSubtitle: "Operator and admin registration is created by an admin from the dashboard, not here.",
  fullNameLabel: "Full name",
  fullNamePlaceholder: "Your name",
  phoneLabel: "Phone number",
  phonePlaceholder: "08xxxxxxxxxx",
  registerPasswordPlaceholder: "At least 8 characters",
  programLabel: "Level / program",
  programPlaceholder: "Choose a level",
  submitRegister: "Create Account",
  haveAccount: "Already have an account?",
  loginLink: "Log In",
  toastAccountCreatedTitle: "Account created successfully",
  toastAccountCreatedDescription: "Welcome! Let's find your first tryout.",

  // forgot-password
  forgotTitle: "Forgot password",
  forgotSubtitle: "Enter your account email — we'll send a link to reset your password.",
  sentTitle: "Check your email",
  sentDescriptionBefore: "If",
  sentDescriptionAfter: "is registered, we've sent a link to reset your password.",
  submitReset: "Send Reset Link",
};

export function useAuthT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
