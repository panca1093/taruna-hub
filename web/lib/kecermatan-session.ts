"use client";

import { computeKecermatanScores, getKecermatanTest } from "./mock-data";
import type {
  KecermatanKolomResult,
  KecermatanReferenceItem,
  KecermatanResult,
  KecermatanSoalAnswer,
  KecermatanTest,
} from "./types";

export interface KecermatanSessionState {
  testId: string;
  userId: string;
  attemptNumber: number;
  status: "in_progress" | "completed";
  startedAt: string;
  completedAt?: string;
  currentKolom: number;
  kolomStartedAt: Record<number, number>;
  kolomCompleted: Record<number, boolean>;
  soalByKolom: Record<number, KecermatanSoalAnswer[]>;
}

const key = (testId: string, userId: string) => `kecermatan-session:${userId}:${testId}`;
const archiveKey = (testId: string, userId: string) => `kecermatan-attempts:${userId}:${testId}`;

export function loadSession(testId: string, userId: string): KecermatanSessionState | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(key(testId, userId));
  if (!raw) return null;
  try {
    return JSON.parse(raw) as KecermatanSessionState;
  } catch {
    return null;
  }
}

export function getAttempts(testId: string, userId: string): KecermatanSessionState[] {
  if (typeof window === "undefined") return [];
  const raw = window.localStorage.getItem(archiveKey(testId, userId));
  if (!raw) return [];
  try {
    return JSON.parse(raw) as KecermatanSessionState[];
  } catch {
    return [];
  }
}

export function remainingAttempts(testId: string, userId: string, purchased: number) {
  return Math.max(0, purchased - getAttempts(testId, userId).length);
}

function archiveState(state: KecermatanSessionState) {
  const attempts = getAttempts(state.testId, state.userId);
  window.localStorage.setItem(archiveKey(state.testId, state.userId), JSON.stringify([...attempts, state]));
}

function save(state: KecermatanSessionState) {
  window.localStorage.setItem(key(state.testId, state.userId), JSON.stringify(state));
}

/* ---------------------------------------------------------------------- */
/* Reference-set & soal generation                                        */
/* ---------------------------------------------------------------------- */

const CHAR_POOL = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789".split(""); // no 0/O, 1/I look-alikes
const SYMBOL_POOL = ["★", "◆", "▲", "●", "■", "♠", "♣", "♥", "♦", "☀", "✦", "✚", "✕", "☰", "◐"];
const EMOJI_POOL = ["🍎", "🍊", "🍇", "🍉", "🍋", "🍓", "🥝", "🍒", "🍑", "🍍", "🥥", "🍌", "🥑", "🍈", "🍐"];
const DIGITS = "0123456789".split("");

function randomItem<T>(pool: T[]): T {
  return pool[Math.floor(Math.random() * pool.length)];
}

function shuffle<T>(items: T[]): T[] {
  const arr = [...items];
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
}

function generateValue(test: KecermatanTest): string {
  const len = test.itemLength;
  switch (test.itemType) {
    case "char":
      return Array.from({ length: len }, () => randomItem(CHAR_POOL)).join("");
    case "number":
      return Array.from({ length: len }, () => randomItem(DIGITS)).join("");
    case "symbol":
      return Array.from({ length: len }, () => randomItem(SYMBOL_POOL)).join(" ");
    case "emoji":
      return Array.from({ length: len }, () => randomItem(EMOJI_POOL)).join("");
    case "mix":
    default: {
      const pools = [CHAR_POOL, DIGITS, SYMBOL_POOL];
      return Array.from({ length: len }, () => randomItem(randomItem(pools))).join("");
    }
  }
}

function generateReferenceSet(test: KecermatanTest): KecermatanReferenceItem[] {
  const seen = new Set<string>();
  return Array.from({ length: test.optionsPerSoal }, (_, i) => {
    const label = String.fromCharCode(65 + i);
    let value = generateValue(test);
    let guard = 0;
    while (seen.has(value) && guard < 25) {
      value = generateValue(test);
      guard += 1;
    }
    seen.add(value);
    return { label, value };
  });
}

function generateSoal(
  referenceSet: KecermatanReferenceItem[],
  kolomNumber: number,
  soalNumber: number,
): KecermatanSoalAnswer {
  const missing = randomItem(referenceSet);
  const remaining = referenceSet.filter((item) => item.label !== missing.label);
  return {
    kolomNumber,
    soalNumber,
    referenceSet,
    displayed: shuffle(remaining),
    missingLabel: missing.label,
    selectedLabel: null,
    isCorrect: false,
  };
}

function generateKolomSoal(test: KecermatanTest, kolomNumber: number): KecermatanSoalAnswer[] {
  const referenceSet = generateReferenceSet(test);
  return Array.from({ length: test.soalPerKolom }, (_, i) => generateSoal(referenceSet, kolomNumber, i + 1));
}

/* ---------------------------------------------------------------------- */
/* Session lifecycle                                                      */
/* ---------------------------------------------------------------------- */

function freshState(testId: string, userId: string, attemptNumber: number): KecermatanSessionState {
  return {
    testId,
    userId,
    attemptNumber,
    status: "in_progress",
    startedAt: new Date().toISOString(),
    currentKolom: 1,
    kolomStartedAt: {},
    kolomCompleted: {},
    soalByKolom: {},
  };
}

export function initSession(testId: string, userId: string): KecermatanSessionState {
  const existing = loadSession(testId, userId);
  if (existing) return existing;
  const state = freshState(testId, userId, 1);
  save(state);
  return state;
}

export function startNewAttempt(testId: string, userId: string): KecermatanSessionState {
  const attemptNumber = getAttempts(testId, userId).length + 1;
  const state = freshState(testId, userId, attemptNumber);
  save(state);
  return state;
}

export function startKolom(testId: string, userId: string, kolomNumber: number): KecermatanSessionState {
  const test = getKecermatanTest(testId);
  const state = initSession(testId, userId);
  if (!state.soalByKolom[kolomNumber] && test) {
    state.soalByKolom[kolomNumber] = generateKolomSoal(test, kolomNumber);
  }
  if (!state.kolomStartedAt[kolomNumber]) {
    state.kolomStartedAt[kolomNumber] = Date.now();
  }
  save(state);
  return state;
}

export function answerSoal(
  testId: string,
  userId: string,
  kolomNumber: number,
  soalNumber: number,
  label: string | null,
): KecermatanSessionState {
  const state = initSession(testId, userId);
  const soal = state.soalByKolom[kolomNumber]?.find((s) => s.soalNumber === soalNumber);
  if (soal) {
    soal.selectedLabel = label;
    soal.isCorrect = label === soal.missingLabel;
  }
  save(state);
  return state;
}

/** Unanswered soal when a kolom's timer expires stay `kosong` (selectedLabel null), not wrong. */
export function completeKolom(testId: string, userId: string, kolomNumber: number): KecermatanSessionState {
  const test = getKecermatanTest(testId);
  const state = initSession(testId, userId);
  state.kolomCompleted[kolomNumber] = true;

  if (test && kolomNumber >= test.kolomCount) {
    state.status = "completed";
    state.completedAt = new Date().toISOString();
    archiveState(state);
  } else {
    state.currentKolom = kolomNumber + 1;
  }
  save(state);
  return state;
}

export function nextPendingKolom(test: KecermatanTest, state: KecermatanSessionState): number | null {
  for (let i = 1; i <= test.kolomCount; i += 1) {
    if (!state.kolomCompleted[i]) return i;
  }
  return null;
}

export function computeResult(test: KecermatanTest, state: KecermatanSessionState): Omit<KecermatanResult, "id"> {
  const kolomResults: KecermatanKolomResult[] = Array.from({ length: test.kolomCount }, (_, i) => {
    const kolomNumber = i + 1;
    const soal = state.soalByKolom[kolomNumber] ?? [];
    const benar = soal.filter((s) => s.selectedLabel !== null && s.isCorrect).length;
    const salah = soal.filter((s) => s.selectedLabel !== null && !s.isCorrect).length;
    return { kolomNumber, benar, salah, kosong: Math.max(0, test.soalPerKolom - benar - salah) };
  });

  return {
    userId: state.userId,
    kecermatanTestId: test.id,
    attemptNumber: state.attemptNumber,
    kolomResults,
    benarTotal: kolomResults.reduce((s, k) => s + k.benar, 0),
    salahTotal: kolomResults.reduce((s, k) => s + k.salah, 0),
    kosongTotal: kolomResults.reduce((s, k) => s + k.kosong, 0),
    completedAt: state.completedAt ?? new Date().toISOString(),
    ...computeKecermatanScores(kolomResults, test.kolomCount * test.soalPerKolom),
  };
}
