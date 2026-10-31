"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  pageTitle: "Persiapan Subtes",
  loading: "Memuat subtes…",
  loadErrorTitle: "Subtes tidak dapat dimuat",
  loadErrorDescription: "Periksa koneksi kamu lalu coba lagi.",
  retry: "Coba Lagi",
  noAccess: "Kamu belum memiliki akses ke tryout ini.",
  backToDetail: "Kembali ke detail",
  startFailed: "Tidak dapat memulai subtes",
  afterBanner: (name: string) => `${name} sudah dikumpulkan dan terkunci.`,
  heading: "Satu subtes pada satu waktu.",
  firstHeading: "Siap memulai tryout?",
  nextHeading: "Siap untuk subtes berikutnya?",
  rule1: "Pilih atau isi jawaban, lalu navigasikan soal sesuka kamu.",
  rule2: "Kamu bisa mengubah jawaban sebelum mengumpulkan subtes.",
  rule3: "Subtes yang dikumpulkan atau kehabisan waktu tidak dapat dibuka kembali.",
  rule4: "Waktu subtes dihitung server dan terus berjalan, termasuk saat kamu memuat ulang halaman atau koneksi terputus.",
  rule5: "Browser akan masuk mode layar penuh. Keluar dari layar penuh dan percobaan screenshot tercatat sebagai pelanggaran.",
  checkTitle: "Periksa sebelum mulai",
  subtestMeta: (min: number, soal: number) => `${soal} soal · ${min} menit`,
  clockNote: "Hitung mundur dimulai saat kamu menekan tombol di bawah.",
  confirmCheck: "Saya sudah membaca petunjuk dan siap mengerjakan.",
  startButton: "Mulai subtes",
  starting: "Memulai…",
  unavailableTitle: "Subtes ini belum bisa dimulai",
  unavailableDescription: "Kerjakan subtes secara berurutan dari halaman detail tryout.",
  lockedTitle: "Subtes ini sudah terkunci",
  lockedDescription: "Subtes yang sudah dikumpulkan atau habis waktunya tidak dapat dibuka kembali.",
};

const en: typeof id = {
  pageTitle: "Subtest Preparation",
  loading: "Loading subtest…",
  loadErrorTitle: "The subtest could not be loaded",
  loadErrorDescription: "Check your connection and try again.",
  retry: "Retry",
  noAccess: "You don't have access to this tryout.",
  backToDetail: "Back to detail",
  startFailed: "Could not start the subtest",
  afterBanner: (name: string) => `${name} was submitted and is now locked.`,
  heading: "One subtest at a time.",
  firstHeading: "Ready to start the tryout?",
  nextHeading: "Ready for the next subtest?",
  rule1: "Pick or type your answers and move between questions freely.",
  rule2: "You can change answers until you submit the subtest.",
  rule3: "A submitted or expired subtest can't be reopened.",
  rule4: "The subtest clock is kept by the server and keeps running, including when you reload or lose connection.",
  rule5: "The browser enters fullscreen. Leaving fullscreen and screenshot attempts are recorded as violations.",
  checkTitle: "Check before you start",
  subtestMeta: (min: number, soal: number) => `${soal} questions · ${min} min`,
  clockNote: "The countdown starts when you press the button below.",
  confirmCheck: "I've read the instructions and I'm ready.",
  startButton: "Start subtest",
  starting: "Starting…",
  unavailableTitle: "This subtest can't be started yet",
  unavailableDescription: "Work through the subtests in order from the tryout detail page.",
  lockedTitle: "This subtest is locked",
  lockedDescription: "A subtest that was submitted or ran out of time can't be reopened.",
};

export function useTryoutPrepareT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
