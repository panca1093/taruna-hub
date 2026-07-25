export type Role = "student" | "operator" | "admin";

export type ProgramCode = "SD" | "SMP" | "SMA" | "CASN" | "BUMN" | "AKPOL";

export interface ProgramSubject {
  id: string;
  name: string;
  chapters: string[];
}

export interface Program {
  id: string;
  /** Base codes are ProgramCode, but admin can create programs with new codes. */
  code: string;
  name: string;
  description: string;
  subjects: ProgramSubject[];
}

export type QuestionType = "pg" | "pg_multi" | "isian" | "benar_salah";
export type Difficulty = "easy" | "medium" | "hard";

export interface QuestionOption {
  id: string;
  label: string;
  /** Per-option point value, used only when the question's `optionWeighted` flag is on. */
  points?: number;
}

export interface Question {
  id: string;
  programId: string;
  subject: string;
  chapter?: string;
  type: QuestionType;
  content: string;
  hasLatex?: boolean;
  imageUrl?: string;
  options?: QuestionOption[];
  correctAnswer: string[];
  difficulty: Difficulty;
  weight: number;
  explanation: string;
  /** All options are valid; score = the chosen option's `points` (missing points default to 1). */
  optionWeighted?: boolean;
}

export type ScoringMode = "weighted" | "irt";

export interface Subtest {
  id: string;
  tryoutId: string;
  name: string;
  durationSeconds: number;
  order: number;
  questionIds: string[];
}

export interface Tryout {
  id: string;
  programId: string;
  title: string;
  description: string;
  coverFrom: string;
  coverTo: string;
  scoringMode: ScoringMode;
  activeFrom?: string;
  activeUntil: string;
  /** If set, pembahasan stays hidden (only score shown) until this date. */
  explanationReleaseAt?: string;
  /** Shuffle question order per attempt (FR-TRY-14). Order is fixed once per session. */
  randomizeQuestions?: boolean;
  /** Shuffle answer-option order for pg/pg_multi per attempt (FR-TRY-15). Option points follow content, not position. */
  randomizeOptions?: boolean;
  subtestIds: string[];
  participantCount: number;
}

export type ProductType = "individual" | "bundle" | "kecermatan";

export interface Product {
  id: string;
  type: ProductType;
  title: string;
  description: string;
  programId: string;
  price: number;
  originalPrice: number;
  activeUntil: string;
  isActive: boolean;
  tryoutIds: string[];
  coverFrom: string;
  coverTo: string;
  /** kecermatan products only. */
  kecermatanTestId?: string;
  includedAttempts?: number;
}

export type OrderStatus = "pending" | "confirmed" | "cancelled";
export type PaymentMode = "gateway" | "manual" | "wallet";

export interface Order {
  id: string;
  code: string;
  userId: string;
  productId: string;
  status: OrderStatus;
  paymentMode: PaymentMode;
  paymentProofUrl?: string;
  amount: number;
  confirmedAt?: string;
  createdAt: string;
}

export interface Enrollment {
  id: string;
  userId: string;
  tryoutId: string;
  orderId: string;
  activeUntil: string;
  createdAt: string;
}

export type WalletTransactionType = "topup" | "deduction";

export interface WalletTransaction {
  id: string;
  userId: string;
  type: WalletTransactionType;
  /** Always positive; `type` carries the direction. */
  amount: number;
  description: string;
  orderId?: string;
  createdAt: string;
}

export type ViolationType = "exit_fullscreen" | "screenshot_attempt";

export interface Violation {
  id: string;
  sessionId: string;
  type: ViolationType;
  timestamp: string;
}

export type SubtestStatus = "pending" | "in_progress" | "completed";

export interface SubtestProgress {
  subtestId: string;
  status: SubtestStatus;
  startedAt?: string;
  completedAt?: string;
  answers: Record<string, string[]>;
}

export type SessionStatus = "in_progress" | "completed";

export interface TryoutSession {
  id: string;
  userId: string;
  tryoutId: string;
  status: SessionStatus;
  startedAt: string;
  completedAt?: string;
  currentSubtestId?: string;
  progress: SubtestProgress[];
  violationCount: number;
}

export interface SubtestScore {
  subtestId: string;
  correct: number;
  total: number;
  score: number;
}

export interface TryoutResult {
  id: string;
  sessionId: string;
  userId: string;
  tryoutId: string;
  attemptNumber: number;
  totalScore: number;
  maxScore: number;
  scoringMode: ScoringMode;
  subtestScores: SubtestScore[];
  rank: number;
  participantCount: number;
  irtWarning: boolean;
  completedAt: string;
}

export interface AppUser {
  id: string;
  name: string;
  email: string;
  phone?: string;
  role: Role;
  program?: ProgramCode;
  createdAt: string;
  avatarColorFrom: string;
  avatarColorTo: string;
  /** Wallet balance (IDR), students only. */
  balance?: number;
}

/* ---------------------------------------------------------------------- */
/* Tes Kecermatan — separate exam format, own product/enrollment model    */
/* ---------------------------------------------------------------------- */

export type KecermatanItemType = "char" | "number" | "symbol" | "emoji" | "mix";

export interface KecermatanTest {
  id: string;
  programId: string;
  title: string;
  description: string;
  coverFrom: string;
  coverTo: string;
  itemType: KecermatanItemType;
  itemLength: number;
  /** Size of the labeled reference set per kolom (also the answer-choice count per soal). */
  optionsPerSoal: number;
  soalPerKolom: number;
  kolomCount: number;
  kolomDurationSeconds: number;
  participantCount: number;
}

/** Metered access: a purchase grants attempts, not a date-bound access window. */
export interface KecermatanEnrollment {
  id: string;
  userId: string;
  kecermatanTestId: string;
  orderId: string;
  attemptsPurchased: number;
  attemptsUsed: number;
  createdAt: string;
}

export interface KecermatanReferenceItem {
  label: string;
  value: string;
}

/** One soal: a shuffled subset of the kolom's reference set with exactly one item omitted. */
export interface KecermatanSoalAnswer {
  kolomNumber: number;
  soalNumber: number;
  referenceSet: KecermatanReferenceItem[];
  displayed: KecermatanReferenceItem[];
  missingLabel: string;
  /** null = kosong (unanswered when kolom timer expired). */
  selectedLabel: string | null;
  isCorrect: boolean;
}

export interface KecermatanKolomResult {
  kolomNumber: number;
  benar: number;
  salah: number;
  kosong: number;
}

export interface KecermatanResult {
  id: string;
  userId: string;
  kecermatanTestId: string;
  attemptNumber: number;
  kolomResults: KecermatanKolomResult[];
  benarTotal: number;
  salahTotal: number;
  kosongTotal: number;
  /** 0-100 composite scores. Formula per PRD §10 Open Question #1 — provisional, not client-validated. */
  kecepatanScore: number;
  ketahananScore: number;
  ketelitianScore: number;
  nilaiAkhirScore: number;
  completedAt: string;
}
