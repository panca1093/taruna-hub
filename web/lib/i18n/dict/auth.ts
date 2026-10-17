"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  // shared
  emailOrPhoneLabel: "Email atau No. HP",
  passwordLabel: "Kata sandi",
  passwordPlaceholder: "••••••••",
  emailLabel: "Email",
  emailPlaceholder: "nama@email.com",
  networkError: "Tidak dapat terhubung ke server. Coba lagi.",
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
  staffEmailOrPhonePlaceholder: "nama@tarunaeduka.id atau 08xxxxxxxxxx",
  submitStaff: "Masuk ke portal internal",
  studentPrompt: "Siswa?",
  studentLoginLink: "Masuk ke akun siswa",

  // register
  registerTitle: "Daftar sebagai siswa",
  registerSubtitle: "Registrasi operator dan admin dibuat oleh admin dari dashboard, bukan di sini.",
  fullNameLabel: "Nama lengkap",
  fullNamePlaceholder: "Nama kamu",
  phoneLabel: "No. HP (opsional)",
  phonePlaceholder: "08xxxxxxxxxx",
  registerPasswordPlaceholder: "Minimal 8 karakter",
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

  // reset-password
  resetTitle: "Atur ulang kata sandi",
  resetSubtitle: "Masukkan kata sandi baru untuk akunmu.",
  newPasswordLabel: "Kata sandi baru",
  confirmPasswordLabel: "Ulangi kata sandi",
  passwordMismatch: "Kata sandi tidak sama.",
  submitNewPassword: "Simpan Kata Sandi",
  resetDoneTitle: "Kata sandi diperbarui",
  resetDoneDescription: "Semua sesi lama sudah diakhiri. Masuk dengan kata sandi barumu.",
  resetLinkInvalid: "Tautan tidak valid atau sudah kedaluwarsa.",
  requestNewLink: "Minta tautan baru",

  // email verification banner
  verifyTitle: "Email belum diverifikasi",
  verifyDescription: (email: string) => `Masukkan kode 6 digit yang kami kirim ke ${email}.`,
  codeLabel: "Kode verifikasi",
  submitCode: "Verifikasi",
  resendCode: "Kirim ulang kode",
  verifiedToast: "Email berhasil diverifikasi",
  resentToast: "Kode baru sudah dikirim",
};

const en: typeof id = {
  // shared
  emailOrPhoneLabel: "Email or Phone Number",
  passwordLabel: "Password",
  passwordPlaceholder: "••••••••",
  emailLabel: "Email",
  emailPlaceholder: "name@email.com",
  networkError: "Cannot reach the server. Try again.",
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
  staffEmailOrPhonePlaceholder: "name@tarunaeduka.id or 08xxxxxxxxxx",
  submitStaff: "Log in to the internal portal",
  studentPrompt: "Student?",
  studentLoginLink: "Log in to student account",

  // register
  registerTitle: "Sign up as a student",
  registerSubtitle: "Operator and admin registration is created by an admin from the dashboard, not here.",
  fullNameLabel: "Full name",
  fullNamePlaceholder: "Your name",
  phoneLabel: "Phone number (optional)",
  phonePlaceholder: "08xxxxxxxxxx",
  registerPasswordPlaceholder: "At least 8 characters",
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

  // reset-password
  resetTitle: "Reset your password",
  resetSubtitle: "Enter a new password for your account.",
  newPasswordLabel: "New password",
  confirmPasswordLabel: "Repeat password",
  passwordMismatch: "Passwords do not match.",
  submitNewPassword: "Save Password",
  resetDoneTitle: "Password updated",
  resetDoneDescription: "All earlier sessions have ended. Log in with your new password.",
  resetLinkInvalid: "This link is invalid or has expired.",
  requestNewLink: "Request a new link",

  // email verification banner
  verifyTitle: "Email not verified",
  verifyDescription: (email: string) => `Enter the 6-digit code we sent to ${email}.`,
  codeLabel: "Verification code",
  submitCode: "Verify",
  resendCode: "Resend code",
  verifiedToast: "Email verified",
  resentToast: "A new code is on its way",
};

export function useAuthT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
