"use client";

import { useLocale, type Locale } from "@/lib/i18n/locale-context";

const id = {
  skip: "Lewati ke konten",
  draftBanner: "Preview draf",
  draftBannerBody: "Menampilkan draf terakhir yang disimpan. Halaman terbit belum berubah.",

  heroKicker: "Taruna Eduka · TNI, POLRI, Kedinasan",
  fallbackHeroTitle: "Persiapanmu, sesuai caramu.",
  fallbackHeroBody:
    "Kenali program bimbel dan diklat TNI, POLRI, atau Kedinasan. Lengkapi persiapanmu dengan tryout bertimer dan latihan Kecermatan dalam satu ruang belajar.",
  fallbackProgramCta: "Kenali program bimbel",
  fallbackPracticeCta: "Pilih latihan mandiri",
  facts: ["Program bimbel & diklat", "Tryout & Kecermatan", "Satu ruang belajar"],

  stageNote: "Cuplikan tampilan · ilustrasi",
  stageTabsLabel: "Jalur persiapan",
  stageTabProgram: "Bimbel & diklat",
  stageTabPractice: "Tryout & Kecermatan",
  stageProgramTitle: "Pilih jalur program",
  stageProgramBody: "Buka gambaran program, lalu kenali alurnya.",
  stageProgramRoute: "Jalur program",
  stageProgramFoot: "Informasi program mengikuti materi resmi Taruna Eduka.",
  stageProgramCta: "Lihat program",
  stagePracticeSwitchLabel: "Jenis latihan",
  stagePracticeTryout: "Tryout",
  stagePracticeKecermatan: "Kecermatan",
  stageTryoutType: "Penalaran numerik",
  stageTryoutQuestion: "25% dari 80 adalah…",
  stageKecermatanBody: "Referensi mana yang memuat item yang hilang?",
  stageIllustrationNote: "Contoh ilustrasi, bukan soal ujian dan tidak memakai kuota.",
  stagePracticeCta: "Lihat paket",
  stageTimerLabel: "Contoh timer",

  chooseHeading: "Pilih jalur persiapanmu.",
  chooseBody: "Kenali program yang sesuai tujuanmu, atau mulai dari latihan mandiri. Keduanya menjadi pintu masuk yang sama penting.",
  entryProgramKicker: "Bimbel & diklat",
  entryProgramHeading: "Kenali programnya.",
  entryProgramBody: "Jelajahi jalur persiapan TNI, POLRI, dan Kedinasan, lalu lihat informasi programnya.",
  entryProgramMeta: "TNI · POLRI · Kedinasan",
  entryProgramCta: "Lihat program bimbel",
  entryPracticeKicker: "Tryout & Kecermatan",
  entryPracticeHeading: "Mulai latihannya.",
  entryPracticeBody: "Pilih tryout untuk penguasaan materi atau Kecermatan untuk ketelitian. Paket latihan dapat dijelajahi secara mandiri.",
  entryPracticeMeta: "Tryout bertimer · latihan per kolom",
  entryPracticeCta: "Lihat paket latihan",

  programsHeading: "Kenali program sesuai tujuanmu.",
  programsBody: "Tiga jalur program Taruna Eduka. Buka detail untuk melihat informasi lengkap setiap jalur.",
  programKicker: "Jalur program",
  programSlots: { tni: "Persiapan TNI", polri: "Persiapan POLRI", kedinasan: "Persiapan Kedinasan" },
  programLink: "Lihat program",
  programScheduleLabel: "Jadwal",
  programLocationLabel: "Lokasi",
  programFeesLabel: "Biaya program",
  programContactLabel: "Kontak",
  programPending: "Menunggu informasi resmi",
  programsNote: "Biaya program hanya informasi. Harga paket latihan selalu mengikuti katalog.",
  programsUnavailableTitle: "Informasi program belum tersedia",
  programsUnavailableBody: "Kami belum bisa memuat informasi program saat ini. Katalog latihan dan login tetap bisa dipakai.",
  programsRetry: "Coba lagi",

  packagesHeading: "Mulai dari latihan yang pas.",
  packagesBody: "Paket latihan yang tersedia saat ini, lengkap dengan harga dari katalog.",
  packagesEmpty: "Katalog sedang disiapkan. Silakan cek lagi nanti.",
  packagesError: "Katalog belum bisa dimuat.",
  packagesRetry: "Coba lagi",
  packagesViewAll: "Lihat semua katalog",
  packagesDetail: "Lihat detail",
  packagesLoading: "Memuat paket latihan",
  packageTypes: { individual: "Tryout", bundle: "Paket tryout", kecermatan: "Kecermatan" },
  packageAttempts: (count: number) => `${count} attempt`,
  packageTryouts: (count: number) => `${count} tryout`,

  workspaceHeading: "Selesai latihan, tahu langkah berikutnya.",
  workspaceBody: "Paket yang kamu miliki, hasil latihan, dan saldo ada dalam satu ruang belajar. Cukup tiga menu utama untuk menemukan yang kamu butuhkan.",
  workspaceCta: "Buat akun",
  workspaceLabel: "Cuplikan ruang belajar siswa · ilustrasi",
  workspaceBrand: "Taruna Eduka",
  workspaceGreeting: "Halo!",
  workspaceGreetingBody: "Selangkah lebih siap, mulai dari hari ini.",
  workspaceStats: ["Tryout dimiliki", "Kuota Kecermatan", "Skor terakhir"],
  workspaceContinue: "Lanjutkan persiapanmu",
  workspaceContinueBody: "Latihan yang sudah kamu miliki, siap dikerjakan.",
  workspaceMenu: ["Dashboard", "Tryout Saya", "Katalog"],

  stepsHeading: "Dari memilih latihan sampai melihat perkembangan.",
  steps: [
    { title: "Pilih paketnya", body: "Temukan program dan jenis latihan yang sesuai, lalu selesaikan pembayaran dengan metode yang tersedia." },
    { title: "Kerjakan latihannya", body: "Masuk ke Tryout atau Kecermatan dari dashboard. Setiap jenis tes punya ritme dan timer sesuai konfigurasinya." },
    { title: "Baca hasilnya", body: "Lihat hasil per attempt. Pembahasan tryout mengikuti jadwal rilis; gunakan evaluasinya untuk latihan berikutnya." },
  ],

  faqHeading: "Sebelum mulai, ada yang ingin kamu tahu?",
  faq: [
    {
      q: "Apa bedanya Tryout dan Kecermatan?",
      a: "Tryout berisi soal dalam subtes untuk mengukur penguasaan materi. Kecermatan memakai rangkaian soal per kolom untuk melatih ketelitian. Bentuk soal, timer, dan hasil keduanya mengikuti konfigurasi tes.",
    },
    {
      q: "Setelah membeli, latihan muncul di mana?",
      a: "Setelah pembayaran dikonfirmasi server, akses tryout atau kuota Kecermatan tersedia di akunmu. Pesanan yang masih pending belum membuka akses.",
    },
    {
      q: "Apakah pembahasan langsung terbuka?",
      a: "Hasil dasar tersedia setelah sesi selesai. Pembahasan tryout mengikuti jadwal rilis yang ditentukan untuk tes tersebut.",
    },
    {
      q: "Apakah verifikasi email wajib sebelum latihan?",
      a: "Akun dapat digunakan meskipun email belum diverifikasi. Kamu bisa memasukkan kode email enam digit saat sudah siap; verifikasi tidak memblokir login.",
    },
  ],

  ctaHeading: "Langkah berikutnya bisa dimulai di sini.",
  ctaBody: "Kenali program bimbel atau temukan latihan mandiri yang sesuai tujuanmu.",

  footerLinks: { programs: "Bimbel & diklat", catalog: "Pilih latihan", howItWorks: "Cara kerja", faq: "Tanya jawab", login: "Masuk", register: "Buat akun" },
  footerRights: "© 2026 Taruna Eduka. Semua hak dilindungi.",
  footerPoweredBy: "Powered by Smart and Simple Solution",
  footerTagline: "Pusat Diklat TNI, POLRI, dan Kedinasan.",
};

const en: typeof id = {
  skip: "Skip to content",
  draftBanner: "Draft preview",
  draftBannerBody: "Showing the last saved draft. The published page has not changed.",

  heroKicker: "Taruna Eduka · TNI, POLRI, Civil Service",
  fallbackHeroTitle: "Your preparation, your way.",
  fallbackHeroBody:
    "Explore TNI, POLRI, or Civil Service training programs. Round out your preparation with timed tryouts and Kecermatan practice in one study space.",
  fallbackProgramCta: "Explore training programs",
  fallbackPracticeCta: "Choose self-paced practice",
  facts: ["Training programs", "Tryouts & Kecermatan", "One study space"],

  stageNote: "Preview · illustration",
  stageTabsLabel: "Preparation paths",
  stageTabProgram: "Training programs",
  stageTabPractice: "Tryouts & Kecermatan",
  stageProgramTitle: "Choose a program path",
  stageProgramBody: "Open a program overview, then learn how it works.",
  stageProgramRoute: "Program path",
  stageProgramFoot: "Program information follows official Taruna Eduka material.",
  stageProgramCta: "View program",
  stagePracticeSwitchLabel: "Practice type",
  stagePracticeTryout: "Tryout",
  stagePracticeKecermatan: "Kecermatan",
  stageTryoutType: "Numerical reasoning",
  stageTryoutQuestion: "25% of 80 is…",
  stageKecermatanBody: "Which reference contains the missing item?",
  stageIllustrationNote: "Illustration only, not an exam question and uses no quota.",
  stagePracticeCta: "View packages",
  stageTimerLabel: "Sample timer",

  chooseHeading: "Choose your preparation path.",
  chooseBody: "Explore the program that fits your goal, or start with self-paced practice. Both are equally important ways in.",
  entryProgramKicker: "Training programs",
  entryProgramHeading: "Explore the programs.",
  entryProgramBody: "Browse the TNI, POLRI, and Civil Service preparation paths, then read the program information.",
  entryProgramMeta: "TNI · POLRI · Civil Service",
  entryProgramCta: "View training programs",
  entryPracticeKicker: "Tryouts & Kecermatan",
  entryPracticeHeading: "Start practicing.",
  entryPracticeBody: "Pick a tryout to master the material or Kecermatan for accuracy. Practice packages can be explored on your own.",
  entryPracticeMeta: "Timed tryouts · per-column practice",
  entryPracticeCta: "View practice packages",

  programsHeading: "Explore programs that fit your goal.",
  programsBody: "Three Taruna Eduka program paths. Open a path for its full information.",
  programKicker: "Program path",
  programSlots: { tni: "TNI preparation", polri: "POLRI preparation", kedinasan: "Civil Service preparation" },
  programLink: "View program",
  programScheduleLabel: "Schedule",
  programLocationLabel: "Location",
  programFeesLabel: "Program fees",
  programContactLabel: "Contact",
  programPending: "Awaiting official information",
  programsNote: "Program fees are information only. Practice package prices always follow the catalog.",
  programsUnavailableTitle: "Program information is unavailable",
  programsUnavailableBody: "We can't load program information right now. The practice catalog and login still work.",
  programsRetry: "Try again",

  packagesHeading: "Start with the right practice.",
  packagesBody: "Practice packages available now, with prices from the catalog.",
  packagesEmpty: "The catalog is being prepared. Please check back later.",
  packagesError: "The catalog could not be loaded.",
  packagesRetry: "Try again",
  packagesViewAll: "View full catalog",
  packagesDetail: "View details",
  packagesLoading: "Loading practice packages",
  packageTypes: { individual: "Tryout", bundle: "Tryout bundle", kecermatan: "Kecermatan" },
  packageAttempts: (count: number) => `${count} attempts`,
  packageTryouts: (count: number) => `${count} tryouts`,

  workspaceHeading: "Finish practice, know what's next.",
  workspaceBody: "The packages you own, your results, and your balance live in one study space. Three main menus are all you need to find what you want.",
  workspaceCta: "Create account",
  workspaceLabel: "Student workspace preview · illustration",
  workspaceBrand: "Taruna Eduka",
  workspaceGreeting: "Hello!",
  workspaceGreetingBody: "One step more prepared, starting today.",
  workspaceStats: ["Tryouts owned", "Kecermatan quota", "Latest score"],
  workspaceContinue: "Continue your preparation",
  workspaceContinueBody: "Practice you already own, ready to take.",
  workspaceMenu: ["Dashboard", "My Tryouts", "Catalog"],

  stepsHeading: "From choosing practice to seeing your progress.",
  steps: [
    { title: "Choose a package", body: "Find the program and practice type that fits, then pay with an available method." },
    { title: "Do the practice", body: "Open Tryout or Kecermatan from the dashboard. Each test type has its own pace and timer according to its configuration." },
    { title: "Read your results", body: "See results per attempt. Tryout discussions follow their release schedule; use the evaluation for your next practice." },
  ],

  faqHeading: "Before you start, any questions?",
  faq: [
    {
      q: "What's the difference between Tryout and Kecermatan?",
      a: "A Tryout has questions in subtests to measure how well you know the material. Kecermatan uses a series of per-column questions to train accuracy. The format, timer, and results of both follow the test configuration.",
    },
    {
      q: "Where does my practice appear after I buy?",
      a: "Once the server confirms payment, access to the tryout or Kecermatan quota appears in your account. Orders that are still pending do not open access.",
    },
    {
      q: "Is the discussion available immediately?",
      a: "Basic results are available once the session ends. The tryout discussion follows the release schedule set for that test.",
    },
    {
      q: "Is email verification required before practicing?",
      a: "Your account works even if your email isn't verified. You can enter the six-digit email code whenever you're ready; verification does not block login.",
    },
  ],

  ctaHeading: "Your next step can start here.",
  ctaBody: "Explore a training program or find self-paced practice that fits your goal.",

  footerLinks: { programs: "Training programs", catalog: "Choose practice", howItWorks: "How it works", faq: "Questions", login: "Sign in", register: "Create account" },
  footerRights: "© 2026 Taruna Eduka. All rights reserved.",
  footerPoweredBy: "Powered by Smart and Simple Solution",
  footerTagline: "TNI, POLRI, and Civil Service Training Center.",
};

export type LandingDict = typeof id;

export function landingDict(locale: Locale): LandingDict {
  return locale === "en" ? en : id;
}

export function useLandingT() {
  const { locale } = useLocale();
  return landingDict(locale);
}
