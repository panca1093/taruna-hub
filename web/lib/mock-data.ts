import type {
  AppUser,
  Enrollment,
  TryoutResult,
  Order,
  Product,
  Program,
  ProgramCode,
  ProgramSubject,
  Question,
  QuestionOption,
  Subtest,
  Tryout,
  TryoutSession,
  Violation,
  WalletTransaction,
} from "./types";
import {
  runtimeBalanceDelta,
  runtimeEnrollments,
  runtimeOrders,
  runtimeWalletTransactions,
} from "./demo-store";

/* ---------------------------------------------------------------------- */
/* Programs                                                               */
/* ---------------------------------------------------------------------- */

let subjSeq = 0;
const subj = (name: string, chapters: string[]): ProgramSubject => {
  subjSeq += 1;
  return { id: `subj-${subjSeq}`, name, chapters };
};

export const programs: Program[] = [
  {
    id: "prog-sd",
    code: "SD",
    name: "Sekolah Dasar",
    description: "Latihan tryout untuk siswa SD kelas 4-6.",
    subjects: [
      subj("Matematika", ["Bilangan", "Pecahan", "Bangun Datar"]),
      subj("IPA", ["Makhluk Hidup", "Energi & Perubahannya"]),
      subj("Bahasa Indonesia", ["Membaca Pemahaman", "Tata Bahasa"]),
    ],
  },
  {
    id: "prog-smp",
    code: "SMP",
    name: "Sekolah Menengah Pertama",
    description: "Latihan tryout untuk siswa SMP kelas 7-9.",
    subjects: [
      subj("Matematika", ["Aljabar", "Geometri", "Bilangan Berpangkat"]),
      subj("IPA", ["Zat & Perubahannya", "Klasifikasi Makhluk Hidup"]),
      subj("Bahasa Inggris", ["Tenses", "Vocabulary"]),
    ],
  },
  {
    id: "prog-sma",
    code: "SMA",
    name: "Sekolah Menengah Atas",
    description: "Simulasi UTBK & tryout mapel SMA/SMK.",
    subjects: [
      subj("Matematika", ["Persamaan Kuadrat", "Kalkulus Dasar", "Logaritma"]),
      subj("Fisika", ["Mekanika", "Listrik & Magnet"]),
      subj("Kimia", ["Tabel Periodik", "Reaksi Kimia"]),
      subj("Tes Potensi Skolastik", ["Penalaran Umum", "Pemahaman Bacaan", "Pengetahuan Kuantitatif"]),
    ],
  },
  {
    id: "prog-casn",
    code: "CASN",
    name: "Calon Aparatur Sipil Negara",
    description: "Latihan SKD CPNS: TWK, TIU, TKP.",
    subjects: [
      subj("TWK", ["Pancasila", "UUD 1945", "Bhinneka Tunggal Ika"]),
      subj("TIU", ["Analogi & Silogisme", "Deret Angka", "Berhitung"]),
      subj("TKP", ["Pelayanan Publik", "Kerja Sama Tim", "Integritas"]),
    ],
  },
  {
    id: "prog-bumn",
    code: "BUMN",
    name: "Rekrutmen BUMN",
    description: "Persiapan tes rekrutmen bersama BUMN.",
    subjects: [
      subj("TKD", ["Verbal", "Numerik"]),
      subj("Wawasan Kebangsaan", ["Sejarah Indonesia", "Lembaga Negara"]),
      subj("Bahasa Inggris", ["Grammar", "Reading Comprehension"]),
    ],
  },
  {
    id: "prog-akpol",
    code: "AKPOL",
    name: "Akademi Kepolisian",
    description: "Persiapan seleksi akademik AKPOL.",
    subjects: [subj("PPBN", ["Tri Brata", "Catur Prasetya"]), subj("TIU", ["Aritmatika", "Deret"])],
  },
];

export const programByCode = (code: ProgramCode) => programs.find((p) => p.code === code)!;
export const getProgram = (id: string) => programs.find((p) => p.id === id);

/* ---------------------------------------------------------------------- */
/* Question bank                                                          */
/* ---------------------------------------------------------------------- */

const opts = (labels: string[]): QuestionOption[] =>
  labels.map((label, i) => ({ id: String.fromCharCode(97 + i), label }));

const YN: QuestionOption[] = [
  { id: "benar", label: "Benar" },
  { id: "salah", label: "Salah" },
];

let qSeq = 0;
const q = (
  programId: string,
  subject: string,
  type: Question["type"],
  difficulty: Question["difficulty"],
  content: string,
  explanation: string,
  extra: Partial<Question> = {},
): Question => {
  qSeq += 1;
  const weight = difficulty === "easy" ? 1 : difficulty === "medium" ? 2 : 3;
  return {
    id: `q${qSeq}`,
    programId,
    subject,
    type,
    content,
    explanation,
    difficulty,
    weight,
    correctAnswer: [],
    ...extra,
  };
};

const SD = "prog-sd";
const SMP = "prog-smp";
const SMA = "prog-sma";
const CASN = "prog-casn";
const BUMN = "prog-bumn";
const AKPOL = "prog-akpol";

export const questions: Question[] = [
  // SD
  q(SD, "Matematika", "pg", "easy", "Hasil dari 248 + 179 adalah ...", "248 + 179 = 427.", {
    options: opts(["417", "427", "437", "447"]),
    correctAnswer: ["b"],
  }),
  q(SD, "Matematika", "pg", "easy", "7 × 8 = ...", "Perkalian dasar: 7 × 8 = 56.", {
    options: opts(["54", "56", "58", "64"]),
    correctAnswer: ["b"],
  }),
  q(SD, "Matematika", "isian", "medium", "Keliling persegi dengan panjang sisi 9 cm adalah ... cm", "Keliling persegi = 4 × sisi = 4 × 9 = 36 cm.", {
    correctAnswer: ["36"],
  }),
  q(SD, "IPA", "benar_salah", "easy", "Matahari terbit dari arah timur.", "Pernyataan ini benar — rotasi bumi menyebabkan matahari tampak terbit dari timur.", {
    options: YN,
    correctAnswer: ["benar"],
  }),
  q(SD, "IPA", "pg_multi", "medium", "Berikut ini yang termasuk sumber energi terbarukan adalah ...", "Matahari, angin, dan air adalah sumber energi terbarukan karena dapat diperbarui secara alami.", {
    options: opts(["Matahari", "Angin", "Air", "Batu bara"]),
    correctAnswer: ["a", "b", "c"],
  }),
  q(SD, "Bahasa Indonesia", "pg", "easy", "Sinonim dari kata 'cerdas' adalah ...", "Sinonim (persamaan makna) dari 'cerdas' adalah 'pintar'.", {
    options: opts(["Bodoh", "Pintar", "Malas", "Lambat"]),
    correctAnswer: ["b"],
  }),
  q(SD, "Matematika", "isian", "easy", "Bilangan yang tepat setelah 999 adalah ...", "Urutan bilangan asli setelah 999 adalah 1000.", {
    correctAnswer: ["1000"],
  }),
  q(SD, "Matematika", "pg", "medium", "Ibu membeli 3 lusin telur. Jumlah telur yang dibeli ibu adalah ...", "1 lusin = 12 butir, sehingga 3 lusin = 3 × 12 = 36 butir.", {
    options: opts(["24", "30", "36", "42"]),
    correctAnswer: ["c"],
  }),

  // SMP
  q(SMP, "Matematika", "pg", "medium", "Jika $2x + 5 = 17$, maka nilai $x$ adalah ...", "$2x = 17 - 5 = 12$, sehingga $x = 6$.", {
    hasLatex: true,
    options: opts(["5", "6", "7", "8"]),
    correctAnswer: ["b"],
  }),
  q(SMP, "IPA", "pg_multi", "hard", "Manakah di antara berikut yang termasuk perubahan kimia?", "Kertas dibakar dan besi berkarat menghasilkan zat baru (perubahan kimia). Es mencair dan lilin meleleh adalah perubahan fisika.", {
    options: opts(["Kertas dibakar", "Es mencair", "Besi berkarat", "Lilin meleleh"]),
    correctAnswer: ["a", "c"],
  }),
  q(SMP, "IPA", "benar_salah", "medium", "Fotosintesis pada tumbuhan menghasilkan oksigen.", "Benar — fotosintesis mengubah CO2 dan air menjadi glukosa dan oksigen.", {
    options: YN,
    correctAnswer: ["benar"],
  }),
  q(SMP, "Matematika", "isian", "medium", "Nilai dari $\\sqrt{144}$ adalah ...", "$\\sqrt{144} = 12$ karena $12 \\times 12 = 144$.", {
    hasLatex: true,
    correctAnswer: ["12"],
  }),
  q(SMP, "Bahasa Inggris", "pg", "easy", "The past tense of 'go' is ...", "Bentuk lampau (past tense) dari 'go' adalah 'went'.", {
    options: opts(["goes", "gone", "went", "going"]),
    correctAnswer: ["c"],
  }),
  q(SMP, "Matematika", "pg", "hard", "Luas permukaan kubus dengan panjang rusuk 6 cm adalah ... $cm^2$", "Luas permukaan kubus = $6 \\times s^2 = 6 \\times 36 = 216\\ cm^2$.", {
    hasLatex: true,
    options: opts(["196", "206", "216", "226"]),
    correctAnswer: ["c"],
  }),
  q(SMP, "Bahasa Inggris", "pg_multi", "medium", "Pilih bentuk jamak (plural) yang tepat:", "Bentuk jamak yang benar adalah 'children' (dari child) dan 'mice' (dari mouse).", {
    options: opts(["childs", "children", "mice", "mouses"]),
    correctAnswer: ["b", "c"],
  }),

  // SMA
  q(SMA, "Matematika", "pg", "hard", "Akar-akar persamaan $x^2 - 5x + 6 = 0$ adalah ...", "Faktorkan: $(x-2)(x-3)=0$, sehingga akar-akarnya $x = 2$ dan $x = 3$.", {
    hasLatex: true,
    options: opts(["2 dan 3", "1 dan 6", "-2 dan -3", "2 dan -3"]),
    correctAnswer: ["a"],
  }),
  q(SMA, "Matematika", "isian", "hard", "Nilai dari $\\log_2 32$ adalah ...", "$\\log_2 32 = 5$ karena $2^5 = 32$.", {
    hasLatex: true,
    correctAnswer: ["5"],
  }),
  q(SMA, "Fisika", "pg", "medium", "Satuan SI untuk gaya adalah ...", "Satuan gaya dalam Sistem Internasional (SI) adalah newton (N).", {
    options: opts(["Joule", "Newton", "Watt", "Pascal"]),
    correctAnswer: ["b"],
  }),
  q(SMA, "Kimia", "pg_multi", "hard", "Manakah unsur berikut yang tergolong logam alkali?", "Logam alkali berada pada golongan IA: Natrium (Na), Kalium (K), dan Litium (Li). Kalsium tergolong logam alkali tanah.", {
    options: opts(["Natrium", "Kalium", "Kalsium", "Litium"]),
    correctAnswer: ["a", "b", "d"],
  }),
  q(SMA, "Fisika", "benar_salah", "medium", "Percepatan gravitasi rata-rata di permukaan bumi adalah 9,8 m/s².", "Benar — nilai ini adalah nilai standar percepatan gravitasi bumi yang umum digunakan.", {
    options: YN,
    correctAnswer: ["benar"],
  }),
  q(SMA, "Matematika", "pg", "hard", "Turunan pertama dari $f(x) = 3x^2 + 2x$ adalah ...", "Menggunakan aturan pangkat: $f'(x) = 6x + 2$.", {
    hasLatex: true,
    options: opts(["6x + 2", "3x + 2", "6x", "x^2"]),
    correctAnswer: ["a"],
  }),
  q(SMA, "Tes Potensi Skolastik", "pg", "medium", "Semua siswa yang rajin belajar lulus ujian. Andi lulus ujian. Maka ...", "Ini adalah kekeliruan logika 'affirming the consequent' — lulus ujian tidak otomatis berarti rajin belajar, bisa ada sebab lain.", {
    options: opts(["Andi pasti rajin belajar", "Andi belum tentu rajin belajar", "Andi tidak rajin belajar", "Tidak dapat disimpulkan sama sekali"]),
    correctAnswer: ["b"],
  }),
  q(SMA, "Kimia", "pg", "easy", "Rumus kimia untuk garam dapur adalah ...", "Garam dapur (natrium klorida) memiliki rumus kimia NaCl.", {
    options: opts(["NaCl", "KCl", "CaCl2", "NaOH"]),
    correctAnswer: ["a"],
  }),
  q(SMA, "Fisika", "pg", "medium", "Sebuah benda bermassa 2 kg bergerak dengan percepatan 3 m/s². Gaya yang bekerja pada benda adalah ... N", "Menggunakan hukum Newton II: $F = m \\times a = 2 \\times 3 = 6$ N.", {
    hasLatex: true,
    options: opts(["5", "6", "8", "10"]),
    correctAnswer: ["b"],
  }),
  q(SMA, "Tes Potensi Skolastik", "pg", "medium", "Kata 'OPTIMIS' berlawanan makna (antonim) dengan ...", "Antonim dari 'optimis' (penuh harap) adalah 'pesimis' (mudah putus asa).", {
    options: opts(["Percaya diri", "Yakin", "Pesimis", "Ceria"]),
    correctAnswer: ["c"],
  }),
  q(SMA, "Tes Potensi Skolastik", "isian", "medium", "2, 6, 12, 20, 30, ... Angka selanjutnya dalam deret tersebut adalah ...", "Pola selisih bertambah 2 setiap suku: +4, +6, +8, +10, +12. Maka suku berikutnya 30 + 12 = 42.", {
    correctAnswer: ["42"],
  }),

  // CASN
  q(CASN, "TWK", "benar_salah", "medium", "Pancasila disahkan sebagai dasar negara Republik Indonesia pada 18 Agustus 1945.", "Benar — Pancasila disahkan pada sidang PPKI tanggal 18 Agustus 1945 bersamaan dengan UUD 1945.", {
    options: YN,
    correctAnswer: ["benar"],
  }),
  q(CASN, "TWK", "pg", "medium", "Lambang negara Indonesia adalah ...", "Lambang negara Indonesia adalah Garuda Pancasila.", {
    options: opts(["Garuda Pancasila", "Bhinneka Tunggal Ika", "Merah Putih", "Pancasila"]),
    correctAnswer: ["a"],
  }),
  q(CASN, "TIU", "pg", "hard", "Jika semua A adalah B, dan sebagian B adalah C, maka ...", "Karena hanya sebagian B yang C, tidak semua A pasti C — hanya kemungkinan sebagian A adalah C.", {
    options: opts(["Semua A adalah C", "Sebagian A mungkin C", "Semua C adalah A", "Tidak ada A yang C"]),
    correctAnswer: ["b"],
  }),
  q(CASN, "TIU", "isian", "medium", "8, 11, 15, 20, 26, ... Angka selanjutnya dalam deret tersebut adalah ...", "Pola selisih bertambah 1 setiap suku: +3, +4, +5, +6, +7. Maka suku berikutnya 26 + 7 = 33.", {
    correctAnswer: ["33"],
  }),
  q(CASN, "TKP", "pg", "easy", "Rekan kerja Anda melakukan kesalahan fatal dalam laporan. Sikap paling tepat adalah ...", "Soal TKP tidak memiliki jawaban salah mutlak — tiap opsi diberi bobot sesuai seberapa ideal sikap tersebut. Melaporkan secara objektif ke atasan (bobot tertinggi) adalah sikap paling profesional.", {
    chapter: "Integritas",
    optionWeighted: true,
    options: [
      { id: "a", label: "Membiarkan saja" }, // belum dikategorikan — default bobot 1
      { id: "b", label: "Melaporkan ke atasan secara objektif", points: 4 },
      { id: "c", label: "Menyalahkan di depan umum", points: 1 },
      { id: "d", label: "Memperbaiki sendiri tanpa melapor", points: 3 },
    ],
    correctAnswer: [],
  }),
  q(CASN, "TWK", "pg_multi", "medium", "Berikut ini termasuk hak asasi manusia yang diatur dalam UUD 1945 adalah ...", "UUD 1945 mengatur hak hidup, hak mendapat pendidikan, dan hak beragama sebagai hak asasi manusia. Memaksakan kehendak bukan hak, melainkan pelanggaran.", {
    options: opts(["Hak hidup", "Hak mendapat pendidikan", "Hak memaksakan kehendak", "Hak beragama"]),
    correctAnswer: ["a", "b", "d"],
  }),

  // BUMN
  q(BUMN, "TKD", "pg", "medium", "BUMN merupakan singkatan dari ...", "BUMN adalah singkatan dari Badan Usaha Milik Negara.", {
    options: opts(["Badan Usaha Milik Negara", "Badan Usaha Milik Nasional", "Badan Umum Milik Negara", "Badan Usaha Modal Negara"]),
    correctAnswer: ["a"],
  }),
  q(BUMN, "TKD", "isian", "easy", "Jika harga barang Rp150.000 didiskon 20%, harga setelah diskon adalah Rp ...", "Diskon 20% dari 150.000 = 30.000, sehingga harga akhir = 150.000 - 30.000 = 120.000.", {
    correctAnswer: ["120000"],
  }),
  q(BUMN, "Wawasan Kebangsaan", "benar_salah", "medium", "OJK bertugas mengawasi dan mengatur sektor jasa keuangan di Indonesia.", "Benar — Otoritas Jasa Keuangan (OJK) adalah lembaga independen yang mengawasi sektor jasa keuangan.", {
    options: YN,
    correctAnswer: ["benar"],
  }),
  q(BUMN, "Bahasa Inggris", "pg", "medium", "Choose the correct sentence:", "Untuk subjek tunggal (she), kata kerja bantu negatif yang benar adalah 'doesn't' diikuti kata kerja dasar 'like'.", {
    options: opts(["She don't like coffee", "She doesn't likes coffee", "She doesn't like coffee", "She not like coffee"]),
    correctAnswer: ["c"],
  }),

  // AKPOL
  q(AKPOL, "PPBN", "pg", "medium", "Tri Brata merupakan pedoman hidup bagi anggota ...", "Tri Brata adalah pedoman hidup dan kode etik bagi anggota Kepolisian Republik Indonesia (Polri).", {
    options: opts(["TNI", "Polri", "ASN", "Mahasiswa"]),
    correctAnswer: ["b"],
  }),
  q(AKPOL, "PPBN", "benar_salah", "medium", "Catur Prasetya adalah pedoman kerja bagi anggota Polri.", "Benar — Catur Prasetya berisi 4 komitmen kerja anggota Polri dalam mengabdi kepada masyarakat.", {
    options: YN,
    correctAnswer: ["benar"],
  }),
  q(AKPOL, "TIU", "pg", "hard", "Sebuah mobil menempuh jarak 240 km dalam waktu 3 jam. Kecepatan rata-rata mobil tersebut adalah ... km/jam", "Kecepatan = jarak ÷ waktu = 240 ÷ 3 = 80 km/jam.", {
    options: opts(["60", "70", "80", "90"]),
    correctAnswer: ["c"],
  }),
  q(AKPOL, "TIU", "isian", "medium", "5, 10, 20, 40, 80, ... Angka selanjutnya dalam deret tersebut adalah ...", "Pola deret geometri dengan rasio 2 (dikali 2 setiap suku). Maka suku berikutnya 80 × 2 = 160.", {
    correctAnswer: ["160"],
  }),
];

export const getQuestion = (id: string) => questions.find((qq) => qq.id === id);
export const getQuestionsByProgram = (programId: string) => questions.filter((qq) => qq.programId === programId);

/* ---------------------------------------------------------------------- */
/* Tryouts & subtests                                                     */
/* ---------------------------------------------------------------------- */

export const subtests: Subtest[] = [
  { id: "st-sd1-1", tryoutId: "tryout-sd-1", name: "Matematika", durationSeconds: 1800, order: 1, questionIds: ["q1", "q2", "q3", "q7", "q8"] },
  { id: "st-sd1-2", tryoutId: "tryout-sd-1", name: "IPA & Bahasa Indonesia", durationSeconds: 1500, order: 2, questionIds: ["q4", "q5", "q6"] },

  { id: "st-smp1-1", tryoutId: "tryout-smp-1", name: "Matematika & IPA", durationSeconds: 2100, order: 1, questionIds: ["q9", "q10", "q11", "q12", "q14"] },
  { id: "st-smp1-2", tryoutId: "tryout-smp-1", name: "Bahasa Inggris", durationSeconds: 1200, order: 2, questionIds: ["q13", "q15"] },

  { id: "st-sma1-1", tryoutId: "tryout-sma-1", name: "Matematika", durationSeconds: 1800, order: 1, questionIds: ["q16", "q17", "q21"] },
  { id: "st-sma1-2", tryoutId: "tryout-sma-1", name: "Fisika & Kimia", durationSeconds: 2100, order: 2, questionIds: ["q18", "q19", "q20", "q23", "q24"] },
  { id: "st-sma1-3", tryoutId: "tryout-sma-1", name: "Tes Potensi Skolastik", durationSeconds: 1500, order: 3, questionIds: ["q22", "q25", "q26"] },

  { id: "st-sma2-1", tryoutId: "tryout-sma-2", name: "Matematika", durationSeconds: 1200, order: 1, questionIds: ["q16", "q21"] },
  { id: "st-sma2-2", tryoutId: "tryout-sma-2", name: "Tes Potensi Skolastik", durationSeconds: 1200, order: 2, questionIds: ["q22", "q25"] },

  { id: "st-casn1-1", tryoutId: "tryout-casn-1", name: "TWK", durationSeconds: 1800, order: 1, questionIds: ["q27", "q28", "q32"] },
  { id: "st-casn1-2", tryoutId: "tryout-casn-1", name: "TIU", durationSeconds: 1500, order: 2, questionIds: ["q29", "q30"] },
  { id: "st-casn1-3", tryoutId: "tryout-casn-1", name: "TKP", durationSeconds: 900, order: 3, questionIds: ["q31"] },

  { id: "st-casn2-1", tryoutId: "tryout-casn-2", name: "TWK", durationSeconds: 1500, order: 1, questionIds: ["q27", "q28"] },
  { id: "st-casn2-2", tryoutId: "tryout-casn-2", name: "TIU", durationSeconds: 1200, order: 2, questionIds: ["q29", "q30"] },

  { id: "st-bumn1-1", tryoutId: "tryout-bumn-1", name: "TKD & Wawasan Kebangsaan", durationSeconds: 1800, order: 1, questionIds: ["q33", "q34", "q35"] },
  { id: "st-bumn1-2", tryoutId: "tryout-bumn-1", name: "Bahasa Inggris", durationSeconds: 900, order: 2, questionIds: ["q36"] },

  { id: "st-akpol1-1", tryoutId: "tryout-akpol-1", name: "PPBN", durationSeconds: 1200, order: 1, questionIds: ["q37", "q38"] },
  { id: "st-akpol1-2", tryoutId: "tryout-akpol-1", name: "TIU", durationSeconds: 1500, order: 2, questionIds: ["q39", "q40"] },
];

export const tryouts: Tryout[] = [
  {
    id: "tryout-sd-1",
    programId: SD,
    title: "Tryout SD Semester 1",
    description: "Latihan tryout mencakup Matematika, IPA, dan Bahasa Indonesia untuk siswa SD kelas 6.",
    coverFrom: "#38bdf8",
    coverTo: "#6366f1",
    scoringMode: "weighted",
    activeUntil: "2026-08-15",
    subtestIds: ["st-sd1-1", "st-sd1-2"],
    participantCount: 58,
  },
  {
    id: "tryout-smp-1",
    programId: SMP,
    title: "Tryout SMP Kelas 9",
    description: "Persiapan ujian sekolah mencakup Matematika, IPA, dan Bahasa Inggris.",
    coverFrom: "#22d3ee",
    coverTo: "#4f46e5",
    scoringMode: "weighted",
    activeUntil: "2026-08-20",
    subtestIds: ["st-smp1-1", "st-smp1-2"],
    participantCount: 74,
  },
  {
    id: "tryout-sma-1",
    programId: SMA,
    title: "Simulasi UTBK SMA Saintek",
    description: "Simulasi lengkap UTBK Saintek: Matematika, Fisika & Kimia, dan Tes Potensi Skolastik.",
    coverFrom: "#4f46e5",
    coverTo: "#8b5cf6",
    scoringMode: "irt",
    activeFrom: "2026-07-01",
    activeUntil: "2026-08-10",
    randomizeQuestions: true,
    randomizeOptions: true,
    explanationReleaseAt: "2026-08-11",
    subtestIds: ["st-sma1-1", "st-sma1-2", "st-sma1-3"],
    participantCount: 22,
  },
  {
    id: "tryout-sma-2",
    programId: SMA,
    title: "Simulasi UTBK SMA Saintek #2",
    description: "Simulasi lanjutan dengan variasi soal Matematika dan TPS.",
    coverFrom: "#8b5cf6",
    coverTo: "#d946ef",
    scoringMode: "irt",
    activeUntil: "2026-08-25",
    subtestIds: ["st-sma2-1", "st-sma2-2"],
    participantCount: 15,
  },
  {
    id: "tryout-casn-1",
    programId: CASN,
    title: "Tryout CASN CPNS Paket 1",
    description: "Latihan SKD CPNS lengkap: TWK, TIU, dan TKP sesuai passing grade terbaru.",
    coverFrom: "#f59e0b",
    coverTo: "#ef4444",
    scoringMode: "weighted",
    activeUntil: "2026-09-01",
    subtestIds: ["st-casn1-1", "st-casn1-2", "st-casn1-3"],
    participantCount: 133,
  },
  {
    id: "tryout-casn-2",
    programId: CASN,
    title: "Tryout CASN CPNS Paket 2",
    description: "Paket lanjutan TWK dan TIU dengan variasi soal berbeda.",
    coverFrom: "#f97316",
    coverTo: "#f43f5e",
    scoringMode: "weighted",
    activeUntil: "2026-09-10",
    subtestIds: ["st-casn2-1", "st-casn2-2"],
    participantCount: 89,
  },
  {
    id: "tryout-bumn-1",
    programId: BUMN,
    title: "Tryout BUMN Batch 1",
    description: "Persiapan Tes Kemampuan Dasar, Wawasan Kebangsaan, dan Bahasa Inggris rekrutmen BUMN.",
    coverFrom: "#10b981",
    coverTo: "#0ea5e9",
    scoringMode: "weighted",
    activeUntil: "2026-08-18",
    subtestIds: ["st-bumn1-1", "st-bumn1-2"],
    participantCount: 41,
  },
  {
    id: "tryout-akpol-1",
    programId: AKPOL,
    title: "Tryout AKPOL Gelombang 1",
    description: "Latihan PPBN dan TIU sesuai kisi-kisi seleksi akademik AKPOL.",
    coverFrom: "#0ea5e9",
    coverTo: "#4f46e5",
    scoringMode: "weighted",
    activeUntil: "2026-08-28",
    subtestIds: ["st-akpol1-1", "st-akpol1-2"],
    participantCount: 19,
  },
];

export const getTryout = (id: string) => tryouts.find((t) => t.id === id);
export const getSubtestsForTryout = (tryoutId: string) =>
  subtests.filter((s) => s.tryoutId === tryoutId).sort((a, b) => a.order - b.order);
export const getSubtest = (id: string) => subtests.find((s) => s.id === id);
export const getQuestionsForSubtest = (subtestId: string) => {
  const st = getSubtest(subtestId);
  if (!st) return [];
  return st.questionIds.map((id) => getQuestion(id)).filter((x): x is Question => Boolean(x));
};

/* ---------------------------------------------------------------------- */
/* Products                                                               */
/* ---------------------------------------------------------------------- */

export const products: Product[] = [
  {
    id: "prod-sd-1",
    type: "individual",
    title: "Tryout SD Semester 1",
    description: "1x paket tryout Matematika, IPA, dan Bahasa Indonesia untuk siswa SD.",
    programId: SD,
    price: 25000,
    originalPrice: 35000,
    activeUntil: "2026-08-15",
    isActive: true,
    tryoutIds: ["tryout-sd-1"],
    coverFrom: "#38bdf8",
    coverTo: "#6366f1",
  },
  {
    id: "prod-smp-1",
    type: "individual",
    title: "Tryout SMP Kelas 9",
    description: "1x paket tryout persiapan ujian sekolah SMP.",
    programId: SMP,
    price: 30000,
    originalPrice: 40000,
    activeUntil: "2026-08-20",
    isActive: true,
    tryoutIds: ["tryout-smp-1"],
    coverFrom: "#22d3ee",
    coverTo: "#4f46e5",
  },
  {
    id: "prod-sma-1",
    type: "individual",
    title: "Simulasi UTBK SMA Saintek",
    description: "1x simulasi UTBK lengkap dengan penilaian mode IRT.",
    programId: SMA,
    price: 45000,
    originalPrice: 60000,
    activeUntil: "2026-08-10",
    isActive: true,
    tryoutIds: ["tryout-sma-1"],
    coverFrom: "#4f46e5",
    coverTo: "#8b5cf6",
  },
  {
    id: "prod-casn-1",
    type: "individual",
    title: "Tryout CASN CPNS Paket 1",
    description: "1x paket tryout SKD CPNS: TWK, TIU, TKP.",
    programId: CASN,
    price: 50000,
    originalPrice: 65000,
    activeUntil: "2026-09-01",
    isActive: true,
    tryoutIds: ["tryout-casn-1"],
    coverFrom: "#f59e0b",
    coverTo: "#ef4444",
  },
  {
    id: "prod-bumn-1",
    type: "individual",
    title: "Tryout BUMN Batch 1",
    description: "1x paket tryout rekrutmen bersama BUMN.",
    programId: BUMN,
    price: 45000,
    originalPrice: 55000,
    activeUntil: "2026-08-18",
    isActive: true,
    tryoutIds: ["tryout-bumn-1"],
    coverFrom: "#10b981",
    coverTo: "#0ea5e9",
  },
  {
    id: "prod-akpol-1",
    type: "individual",
    title: "Tryout AKPOL Gelombang 1",
    description: "1x paket tryout seleksi akademik AKPOL.",
    programId: AKPOL,
    price: 45000,
    originalPrice: 55000,
    activeUntil: "2026-08-28",
    isActive: true,
    tryoutIds: ["tryout-akpol-1"],
    coverFrom: "#0ea5e9",
    coverTo: "#4f46e5",
  },
  {
    id: "prod-sma-bundle",
    type: "bundle",
    title: "Bundle Saintek — 2x Tryout UTBK",
    description: "Hemat! Dua paket simulasi UTBK Saintek dengan variasi soal berbeda.",
    programId: SMA,
    price: 75000,
    originalPrice: 105000,
    activeUntil: "2026-08-25",
    isActive: true,
    tryoutIds: ["tryout-sma-1", "tryout-sma-2"],
    coverFrom: "#6366f1",
    coverTo: "#d946ef",
  },
  {
    id: "prod-casn-bundle",
    type: "bundle",
    title: "Bundle CASN Lengkap — 2x Tryout",
    description: "Hemat! Dua paket tryout SKD CPNS untuk latihan maksimal sebelum ujian.",
    programId: CASN,
    price: 85000,
    originalPrice: 115000,
    activeUntil: "2026-09-10",
    isActive: true,
    tryoutIds: ["tryout-casn-1", "tryout-casn-2"],
    coverFrom: "#f97316",
    coverTo: "#f43f5e",
  },
  {
    id: "prod-kec-casn",
    type: "kecermatan",
    title: "Kecermatan Simbol Hilang — TKP CASN",
    description: "3x kesempatan mengerjakan tes kecermatan gaya psikotes untuk latihan TKP CASN.",
    programId: CASN,
    price: 20000,
    originalPrice: 30000,
    activeUntil: "2026-09-01",
    isActive: true,
    tryoutIds: [],
    kecermatanTestId: "kec-casn-1",
    includedAttempts: 3,
    coverFrom: "#f59e0b",
    coverTo: "#ef4444",
  },
  {
    id: "prod-kec-sma",
    type: "kecermatan",
    title: "Kecermatan Simbol Hilang — Persiapan UTBK",
    description: "3x kesempatan latihan ketelitian dan kecepatan sebagai pelengkap simulasi UTBK.",
    programId: SMA,
    price: 20000,
    originalPrice: 28000,
    activeUntil: "2026-08-25",
    isActive: true,
    tryoutIds: [],
    kecermatanTestId: "kec-sma-1",
    includedAttempts: 3,
    coverFrom: "#0ea5e9",
    coverTo: "#4f46e5",
  },
  {
    id: "prod-kec-bumn",
    type: "kecermatan",
    title: "Kecermatan Simbol Hilang — Rekrutmen BUMN",
    description: "2x kesempatan latihan kecermatan simbol untuk persiapan tes rekrutmen BUMN.",
    programId: BUMN,
    price: 18000,
    originalPrice: 25000,
    activeUntil: "2026-08-18",
    isActive: true,
    tryoutIds: [],
    kecermatanTestId: "kec-bumn-1",
    includedAttempts: 2,
    coverFrom: "#10b981",
    coverTo: "#0ea5e9",
  },
];

export const getProduct = (id: string) => products.find((p) => p.id === id);

/* ---------------------------------------------------------------------- */
/* Users                                                                  */
/* ---------------------------------------------------------------------- */

const grad = (from: string, to: string) => ({ avatarColorFrom: from, avatarColorTo: to });
const gradients = [
  grad("#4f46e5", "#8b5cf6"),
  grad("#0ea5e9", "#22d3ee"),
  grad("#10b981", "#34d399"),
  grad("#f59e0b", "#f97316"),
  grad("#f43f5e", "#ec4899"),
  grad("#8b5cf6", "#d946ef"),
];

export const currentStudent: AppUser = {
  id: "u-student-1",
  name: "Naila Ramadhani",
  email: "naila.ramadhani@example.com",
  phone: "081234567890",
  role: "student",
  program: "SMA",
  createdAt: "2026-05-02",
  balance: 150000,
  ...gradients[0],
};

export const currentOperator: AppUser = {
  id: "u-operator-1",
  name: "Dimas Prakoso",
  email: "dimas.prakoso@tarunaeduka.id",
  role: "operator",
  createdAt: "2026-02-10",
  ...gradients[1],
};

export const currentAdmin: AppUser = {
  id: "u-admin-1",
  name: "Siti Aminah",
  email: "siti.aminah@tarunaeduka.id",
  role: "admin",
  createdAt: "2025-11-01",
  ...gradients[2],
};

const studentNames: [string, ProgramCode][] = [
  ["Naila Ramadhani", "SMA"],
  ["Bagas Wirayuda", "SMA"],
  ["Kirana Dewi Anjani", "SMA"],
  ["Fajar Nugroho", "SMP"],
  ["Ayu Lestari", "SMP"],
  ["Rafi Ardiansyah", "SD"],
  ["Zahra Aulia Putri", "SD"],
  ["Yusuf Maulana", "CASN"],
  ["Intan Permatasari", "CASN"],
  ["Bimo Satrio", "CASN"],
  ["Devi Anggraini", "BUMN"],
  ["Hafiz Rahman", "BUMN"],
  ["Salsabila Azzahra", "AKPOL"],
  ["Reza Firmansyah", "AKPOL"],
  ["Putri Wulandari", "SMA"],
  ["Arya Bagaskara", "SMP"],
  ["Nadia Kusuma", "SD"],
  ["Fikri Alamsyah", "CASN"],
  ["Melati Puspita", "BUMN"],
  ["Galih Pratama", "SMA"],
];

export const students: AppUser[] = studentNames.map(([name, program], i) => ({
  id: `u-student-${i + 2}`,
  name,
  email: `${name.toLowerCase().replace(/[^a-z ]/g, "").replace(/\s+/g, ".")}@example.com`,
  role: "student",
  program,
  createdAt: `2026-0${2 + (i % 4)}-${String(3 + ((i * 3) % 24)).padStart(2, "0")}`,
  ...gradients[i % gradients.length],
}));

export const allStudents: AppUser[] = [currentStudent, ...students];

export const operators: AppUser[] = [
  currentOperator,
  {
    id: "u-operator-2",
    name: "Rani Setiawati",
    email: "rani.setiawati@tarunaeduka.id",
    role: "operator",
    createdAt: "2026-03-18",
    ...gradients[3],
  },
];

export const admins: AppUser[] = [currentAdmin];

export const findUser = (id: string) => [...allStudents, ...operators, ...admins].find((u) => u.id === id);

/* ---------------------------------------------------------------------- */
/* Orders & enrollments                                                   */
/* ---------------------------------------------------------------------- */

let orderSeq = 0;
const mkOrder = (
  userId: string,
  productId: string,
  status: Order["status"],
  paymentMode: Order["paymentMode"],
  createdAt: string,
  confirmedAt?: string,
  hasProof?: boolean,
): Order => {
  orderSeq += 1;
  const product = products.find((p) => p.id === productId)!;
  return {
    id: `order-${orderSeq}`,
    code: `TRX-2026-${String(1000 + orderSeq)}`,
    userId,
    productId,
    status,
    paymentMode,
    paymentProofUrl: hasProof ? "/mock/payment-proof.jpg" : undefined,
    amount: product.price,
    createdAt,
    confirmedAt,
  };
};

export const orders: Order[] = [
  mkOrder(currentStudent.id, "prod-sma-1", "confirmed", "manual", "2026-06-10", "2026-06-10T14:22:00", true),
  mkOrder(currentStudent.id, "prod-sma-bundle", "confirmed", "manual", "2026-06-18", "2026-06-19T09:05:00", true),
  mkOrder(currentStudent.id, "prod-casn-1", "pending", "manual", "2026-07-01", undefined, true),
  mkOrder(students[0].id, "prod-sma-1", "confirmed", "manual", "2026-06-05", "2026-06-05T11:00:00", true),
  mkOrder(students[1].id, "prod-sma-1", "confirmed", "manual", "2026-06-06", "2026-06-06T13:40:00", true),
  mkOrder(students[2].id, "prod-smp-1", "confirmed", "gateway", "2026-06-08", "2026-06-08T08:12:00"),
  mkOrder(students[3].id, "prod-smp-1", "pending", "manual", "2026-07-02", undefined, true),
  mkOrder(students[4].id, "prod-sd-1", "confirmed", "manual", "2026-06-12", "2026-06-12T16:30:00", true),
  mkOrder(students[5].id, "prod-sd-1", "cancelled", "manual", "2026-06-01"),
  mkOrder(students[6].id, "prod-casn-1", "confirmed", "manual", "2026-06-15", "2026-06-16T10:00:00", true),
  mkOrder(students[7].id, "prod-casn-bundle", "confirmed", "gateway", "2026-06-20", "2026-06-20T09:45:00"),
  mkOrder(students[8].id, "prod-casn-1", "pending", "manual", "2026-06-30", undefined, true),
  mkOrder(students[9].id, "prod-bumn-1", "confirmed", "manual", "2026-06-22", "2026-06-22T15:10:00", true),
  mkOrder(students[10].id, "prod-bumn-1", "confirmed", "manual", "2026-06-23", "2026-06-24T09:00:00", true),
  mkOrder(students[11].id, "prod-akpol-1", "confirmed", "manual", "2026-06-25", "2026-06-25T14:00:00", true),
  mkOrder(students[12].id, "prod-akpol-1", "pending", "manual", "2026-07-01", undefined, false),
  mkOrder(students[13].id, "prod-sma-1", "confirmed", "manual", "2026-06-14", "2026-06-14T11:20:00", true),
  mkOrder(students[14].id, "prod-smp-1", "cancelled", "manual", "2026-06-09"),
  mkOrder(students[15].id, "prod-sd-1", "confirmed", "gateway", "2026-06-27", "2026-06-27T08:00:00"),
  mkOrder(students[16].id, "prod-casn-1", "confirmed", "manual", "2026-06-28", "2026-06-29T10:15:00", true),
  mkOrder(currentStudent.id, "prod-bumn-1", "confirmed", "wallet", "2026-06-26", "2026-06-26T10:00:00"),
  mkOrder(currentStudent.id, "prod-kec-sma", "confirmed", "gateway", "2026-07-05", "2026-07-05T10:00:00"),
  mkOrder(students[7].id, "prod-kec-casn", "confirmed", "manual", "2026-07-01", "2026-07-01T09:00:00", true),
  mkOrder(students[8].id, "prod-kec-casn", "confirmed", "manual", "2026-07-02", "2026-07-02T11:30:00", true),
  mkOrder(students[10].id, "prod-kec-bumn", "confirmed", "gateway", "2026-07-03", "2026-07-03T08:40:00"),
];

export const getOrdersForUser = (userId: string) => [
  ...orders.filter((o) => o.userId === userId),
  ...runtimeOrders(userId),
];

/** Sentinel productId untuk pesanan top-up saldo (bukan pembelian produk). */
export const TOPUP_PRODUCT_ID = "topup-saldo";
export const getOrderTitle = (order: Order) =>
  order.productId === TOPUP_PRODUCT_ID ? "Top Up Saldo" : (getProduct(order.productId)?.title ?? order.productId);

let enrollSeq = 0;
export const enrollments: Enrollment[] = orders
  .filter((o) => o.status === "confirmed")
  .flatMap((o) => {
    const product = products.find((p) => p.id === o.productId)!;
    return product.tryoutIds.map((tryoutId) => {
      enrollSeq += 1;
      return {
        id: `enroll-${enrollSeq}`,
        userId: o.userId,
        tryoutId,
        orderId: o.id,
        activeUntil: product.activeUntil,
        createdAt: o.confirmedAt ?? o.createdAt,
      } satisfies Enrollment;
    });
  });

export const getEnrollmentsForUser = (userId: string) => [
  ...enrollments.filter((e) => e.userId === userId),
  ...runtimeEnrollments(userId),
];
export const hasEnrollment = (userId: string, tryoutId: string) =>
  getEnrollmentsForUser(userId).some((e) => e.tryoutId === tryoutId);

/* ---------------------------------------------------------------------- */
/* Wallet ledger                                                          */
/* ---------------------------------------------------------------------- */

/**
 * Seeded history; runtime top-ups and purchases are appended from the demo store.
 * Net of these entries must equal `currentStudent.balance`, or the ledger contradicts the balance card.
 */
export const walletTransactions: WalletTransaction[] = [
  {
    id: "wtx-1",
    userId: currentStudent.id,
    type: "topup",
    amount: 200_000,
    description: "Top up saldo — transfer bank",
    createdAt: "2026-06-24T09:15:00",
  },
  {
    id: "wtx-2",
    userId: currentStudent.id,
    type: "deduction",
    amount: 95_000,
    description: "Tryout BUMN Persiapan",
    orderId: "order-21",
    createdAt: "2026-06-26T10:00:00",
  },
  {
    id: "wtx-3",
    userId: currentStudent.id,
    type: "topup",
    amount: 45_000,
    description: "Top up saldo — transfer bank",
    createdAt: "2026-07-08T14:30:00",
  },
];

export const getWalletTransactionsForUser = (userId: string) =>
  [...walletTransactions.filter((w) => w.userId === userId), ...runtimeWalletTransactions(userId)].sort((a, b) =>
    a.createdAt < b.createdAt ? 1 : -1,
  );

/** Seeded balance adjusted by anything spent or topped up during the demo. */
export const getBalance = (user: Pick<AppUser, "id" | "balance">) => (user.balance ?? 0) + runtimeBalanceDelta(user.id);

/* ---------------------------------------------------------------------- */
/* Sessions, violations, and results                                      */
/* ---------------------------------------------------------------------- */

export const violations: Violation[] = [
  { id: "v1", sessionId: "sess-live-1", type: "exit_fullscreen", timestamp: "2026-07-02T09:14:02" },
  { id: "v2", sessionId: "sess-live-1", type: "exit_fullscreen", timestamp: "2026-07-02T09:26:47" },
  { id: "v3", sessionId: "sess-live-3", type: "exit_fullscreen", timestamp: "2026-07-02T09:05:11" },
  { id: "v4", sessionId: "sess-r4", type: "exit_fullscreen", timestamp: "2026-06-21T09:03:15" },
  { id: "v5", sessionId: "sess-r4", type: "screenshot_attempt", timestamp: "2026-06-21T09:11:40" },
];

export const liveSessions: TryoutSession[] = [
  {
    id: "sess-live-1",
    userId: students[0].id,
    tryoutId: "tryout-sma-1",
    status: "in_progress",
    startedAt: "2026-07-02T09:00:00",
    currentSubtestId: "st-sma1-1",
    progress: [],
    violationCount: 2,
  },
  {
    id: "sess-live-2",
    userId: students[9].id,
    tryoutId: "tryout-bumn-1",
    status: "in_progress",
    startedAt: "2026-07-02T09:20:00",
    currentSubtestId: "st-bumn1-1",
    progress: [],
    violationCount: 0,
  },
  {
    id: "sess-live-3",
    userId: students[6].id,
    tryoutId: "tryout-casn-1",
    status: "in_progress",
    startedAt: "2026-07-02T08:55:00",
    currentSubtestId: "st-casn1-2",
    progress: [],
    violationCount: 1,
  },
];

const mkResult = (
  id: string,
  userId: string,
  tryoutId: string,
  totalScore: number,
  maxScore: number,
  rank: number,
  participantCount: number,
  completedAt: string,
  irtWarning = false,
  attemptNumber = 1,
): TryoutResult => {
  const tryout = getTryout(tryoutId)!;
  const subtestList = getSubtestsForTryout(tryoutId);
  const per = Math.floor(totalScore / subtestList.length);
  const ratio = totalScore / maxScore;
  return {
    id,
    sessionId: `sess-${id}`,
    userId,
    tryoutId,
    attemptNumber,
    totalScore,
    maxScore,
    scoringMode: tryout.scoringMode,
    subtestScores: subtestList.map((s, i) => {
      const total = s.questionIds.length;
      const correct = Math.min(total, Math.max(1, Math.round(ratio * total)));
      return {
        subtestId: s.id,
        correct,
        total,
        score: i === subtestList.length - 1 ? totalScore - per * (subtestList.length - 1) : per,
      };
    }),
    rank,
    participantCount,
    irtWarning,
    completedAt,
  };
};

export const tryoutResults: TryoutResult[] = [
  mkResult("r1", students[3].id, "tryout-sma-1", 78, 100, 3, 22, "2026-06-20T11:40:00", true),
  mkResult("r2", students[0].id, "tryout-sma-1", 91, 100, 1, 22, "2026-06-19T12:05:00", true),
  mkResult("r2b", students[0].id, "tryout-sma-1", 84, 100, 2, 22, "2026-06-24T15:10:00", true, 2),
  mkResult("r3", students[1].id, "tryout-sma-1", 85, 100, 2, 22, "2026-06-20T10:30:00", true),
  mkResult("r4", students[13].id, "tryout-sma-1", 66, 100, 5, 22, "2026-06-21T09:15:00", true),
  mkResult("r5", students[6].id, "tryout-casn-1", 82, 100, 2, 133, "2026-06-22T13:00:00"),
  mkResult("r6", students[7].id, "tryout-casn-1", 88, 100, 1, 133, "2026-06-22T14:20:00"),
  mkResult("r7", students[9].id, "tryout-bumn-1", 74, 100, 3, 41, "2026-06-25T10:00:00"),
  mkResult("r8", students[10].id, "tryout-bumn-1", 79, 100, 2, 41, "2026-06-25T10:45:00"),
  mkResult("r9", students[11].id, "tryout-akpol-1", 70, 100, 4, 19, "2026-06-27T09:30:00"),
  mkResult("r10", students[4].id, "tryout-sd-1", 92, 100, 1, 58, "2026-06-14T09:00:00"),
];

export const getResultsForTryout = (tryoutId: string) =>
  tryoutResults.filter((r) => r.tryoutId === tryoutId).sort((a, b) => a.rank - b.rank);
export const getResultsForUser = (userId: string) => tryoutResults.filter((r) => r.userId === userId);

/** Ranking is by each student's average score across all their attempts, not per attempt. */
export const getLeaderboardForTryout = (tryoutId: string) => {
  const rows = tryoutResults.filter((r) => r.tryoutId === tryoutId);
  const byUser = new Map<string, TryoutResult[]>();
  for (const r of rows) {
    byUser.set(r.userId, [...(byUser.get(r.userId) ?? []), r]);
  }
  return [...byUser.entries()]
    .map(([userId, attempts]) => ({
      userId,
      avgScore: Math.round(attempts.reduce((sum, a) => sum + a.totalScore, 0) / attempts.length),
      attemptCount: attempts.length,
    }))
    .sort((a, b) => b.avgScore - a.avgScore)
    .map((row, i) => ({ ...row, rank: i + 1 }));
};

/* ---------------------------------------------------------------------- */
/* Formatting helpers                                                     */
/* ---------------------------------------------------------------------- */

export const formatIDR = (value: number) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value);

export const formatDate = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "long", year: "numeric" }).format(new Date(iso));

export const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

export const initials = (name: string) =>
  name
    .split(" ")
    .map((p) => p[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
