"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  heroBadge: "Telah Hadir!",
  heroTitle: "Program Diklat TNI, Polri, dan Kedinasan terbaik se-Indonesia.",
  heroDescription:
    "Pusat pendidikan dan latihan terbaik untuk persiapan tes TNI, POLRI, dan Kedinasan. Dilengkapi sistem ujian berbasis CAT dengan laporan hasil tes untuk mengukur perkembangan belajar siswa.",
  ctaRegister: "Daftar Sekarang",
  ctaCatalog: "Lihat Katalog Tryout",
  statPrograms: "program diklat",
  statTryouts: "tryout tersedia",
  statInstant: "Instan",
  statScoreDiscussion: "skor & pembahasan",

  programsEyebrow: "Program Diklat Taruna Eduka",
  programsHeading: "Berikut daftar diklat yang telah tersedia",
  diklatPrograms: [
    { title: "Diklat TNI", description: "Program AKMIL, Bintara TNI, Tamtama TNI, AAU, AAL, dan AAD." },
    { title: "Diklat POLRI", description: "Program AKPOL, Bintara POLRI, Tamtama POLRI, dan SIPSS." },
    { title: "Diklat Kedinasan", description: "Program kedinasan paling lengkap: STAN, IPDN, STMKG, STIN, dan lainnya." },
  ],
  diklatCta: "Daftar Sekarang →",

  valuePropsEyebrow: "Keunggulan kami",
  valuePropsHeading: "Satu aplikasi dengan banyak manfaat untuk siswa.",
  valueProps: [
    {
      title: "Soal-soal HOTS",
      description:
        "Soal HOTS yang lengkap dan diperbarui setiap tahun mengikuti perkembangan soal tes sesungguhnya, memastikan kisi-kisi tetap mirip.",
    },
    {
      title: "Tryout sistem CAT",
      description:
        "Dibuat semirip mungkin dengan sistem CAT — timer mundur per subtes dan navigasi bebas antar soal, melatih kontrol waktu ujian.",
    },
    {
      title: "Hasil tes langsung keluar",
      description:
        "Skor langsung keluar dalam hitungan detik setelah selesai tanpa menunggu waktu lama, dilengkapi analisa hasil tryout.",
    },
    {
      title: "Soal bisa di-download",
      description:
        "Soal dan pembahasan tryout dapat diunduh dalam format PDF untuk memudahkan menganalisa dan mempelajari ulang pembahasannya.",
    },
    {
      title: "Analisa super detail",
      description: "Hasil tryout diukur sampai ke butir soal — kamu tahu persis materi mana yang kuat dan mana yang perlu ditingkatkan.",
    },
    {
      title: "Tryout termurah",
      description: "Tryout premium dengan harga paling terjangkau dibanding kompetitor. Kualitasnya sudah dibuktikan langsung oleh peserta bimbel.",
    },
  ],

  featuredEyebrow: "Paket populer",
  featuredHeading: "Mulai dari tryout ini",
  featuredViewAll: "Lihat semua katalog →",
  featuredEmpty: "Katalog sedang disiapkan. Silakan cek lagi nanti.",

  howItWorksEyebrow: "Cara kerja",
  howItWorksHeading: "Tiga langkah, dari beli sampai lihat hasil.",
  steps: [
    { title: "Pilih & beli paket", description: "Cari tryout sesuai jenjangmu di katalog, lalu selesaikan pembayaran." },
    {
      title: "Kerjakan sesuai timer",
      description: "Setiap subtes punya durasi sendiri. Sekali disubmit atau waktu habis, subtes terkunci — sama seperti ujian sungguhan.",
    },
    {
      title: "Lihat hasil & pembahasan",
      description: "Skor, ranking, dan pembahasan setiap soal langsung terbuka setelah subtes terakhir selesai.",
    },
  ],

  ctaHeading: "Masih belum yakin dengan Aplikasi Tryout Taruna Eduka?",
  ctaDescription: "Yuk daftarkan akunmu segera hari ini juga dan kejar impian karier di layanan negara.",

  footerRights: "© 2026 Taruna Eduka. Semua hak dilindungi.",
  footerPoweredBy: "Powered by Smart and Simple Solution",
  footerTagline: "Pusat Diklat TNI, POLRI, dan Kedinasan.",
  footerInternalPortal: "Portal internal (Operator/Admin)",

  heroPreviewResultLabel: "Hasil tryout",
  heroPreviewRank: (rank: number, total: number) => `Peringkat ${rank} dari ${total} peserta`,
  heroPreviewQuestionProgress: (current: number, total: number) => `Soal ${current}/${total}`,
};

const en: typeof id = {
  heroBadge: "Just Launched!",
  heroTitle: "Indonesia's best TNI, Polri, and Civil Service training programs.",
  heroDescription:
    "A top education and training center to prepare for TNI, POLRI, and Civil Service exams. Featuring a CAT-based exam system with result reports to track student progress.",
  ctaRegister: "Register Now",
  ctaCatalog: "View Tryout Catalog",
  statPrograms: "training programs",
  statTryouts: "tryouts available",
  statInstant: "Instant",
  statScoreDiscussion: "score & discussion",

  programsEyebrow: "Taruna Eduka Training Programs",
  programsHeading: "Here are the training programs currently available",
  diklatPrograms: [
    { title: "TNI Training", description: "AKMIL, TNI NCO, TNI Enlisted, AAU, AAL, and AAD programs." },
    { title: "POLRI Training", description: "AKPOL, POLRI NCO, POLRI Enlisted, and SIPSS programs." },
    { title: "Civil Service Training", description: "The most complete civil service programs: STAN, IPDN, STMKG, STIN, and more." },
  ],
  diklatCta: "Register Now →",

  valuePropsEyebrow: "Our advantages",
  valuePropsHeading: "One app, many benefits for students.",
  valueProps: [
    {
      title: "HOTS questions",
      description:
        "Complete HOTS questions updated every year to follow the real exam's development, keeping the blueprint closely aligned.",
    },
    {
      title: "CAT-system tryouts",
      description:
        "Built to closely match the CAT system — a countdown timer per subtest and free navigation between questions, training your exam time control.",
    },
    {
      title: "Instant results",
      description: "Scores come out within seconds of finishing, no long wait, complete with a tryout result analysis.",
    },
    {
      title: "Downloadable questions",
      description: "Tryout questions and discussions can be downloaded as a PDF to make it easier to analyze and review later.",
    },
    {
      title: "Super detailed analysis",
      description: "Tryout results are measured down to each question — you know exactly which topics are strong and which need work.",
    },
    {
      title: "Most affordable tryouts",
      description: "Premium tryouts at the most affordable price compared to competitors. Quality already proven by our students.",
    },
  ],

  featuredEyebrow: "Popular packages",
  featuredHeading: "Start with these tryouts",
  featuredViewAll: "View full catalog →",
  featuredEmpty: "The catalog is being prepared. Please check back later.",

  howItWorksEyebrow: "How it works",
  howItWorksHeading: "Three steps, from purchase to results.",
  steps: [
    { title: "Choose & buy a package", description: "Find a tryout for your level in the catalog, then complete payment." },
    {
      title: "Work within the timer",
      description: "Each subtest has its own duration. Once submitted or time runs out, the subtest locks — just like the real exam.",
    },
    {
      title: "See results & discussion",
      description: "Score, ranking, and the discussion for every question open right after the last subtest ends.",
    },
  ],

  ctaHeading: "Still not sure about the Taruna Eduka Tryout App?",
  ctaDescription: "Register your account today and go after your dream career in public service.",

  footerRights: "© 2026 Taruna Eduka. All rights reserved.",
  footerPoweredBy: "Powered by Smart and Simple Solution",
  footerTagline: "TNI, POLRI, and Civil Service Training Center.",
  footerInternalPortal: "Internal portal (Operator/Admin)",

  heroPreviewResultLabel: "Tryout result",
  heroPreviewRank: (rank: number, total: number) => `Rank ${rank} of ${total} participants`,
  heroPreviewQuestionProgress: (current: number, total: number) => `Question ${current}/${total}`,
};

export function useLandingT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
