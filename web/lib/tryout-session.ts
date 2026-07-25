"use client";

import { getQuestion, getSubtestsForTryout, getTryout } from "./mock-data";
import type { TryoutResult, Question, SubtestScore, SubtestStatus, Tryout, ViolationType } from "./types";

export interface TryoutSessionState {
  tryoutId: string;
  userId: string;
  attemptNumber: number;
  status: "in_progress" | "completed";
  startedAt: string;
  completedAt?: string;
  subtestStatus: Record<string, SubtestStatus>;
  subtestStartedAt: Record<string, number>;
  answers: Record<string, string[]>;
  violationCount: number;
  violationLog: { type: ViolationType; timestamp: string }[];
}

const key = (tryoutId: string, userId: string) => `tryout-session:${userId}:${tryoutId}`;
const archiveKey = (tryoutId: string, userId: string) => `tryout-attempts:${userId}:${tryoutId}`;

export function loadSession(tryoutId: string, userId: string): TryoutSessionState | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(key(tryoutId, userId));
  if (!raw) return null;
  try {
    return JSON.parse(raw) as TryoutSessionState;
  } catch {
    return null;
  }
}

export function getAttempts(tryoutId: string, userId: string): TryoutSessionState[] {
  if (typeof window === "undefined") return [];
  const raw = window.localStorage.getItem(archiveKey(tryoutId, userId));
  if (!raw) return [];
  try {
    return JSON.parse(raw) as TryoutSessionState[];
  } catch {
    return [];
  }
}

function archive(state: TryoutSessionState) {
  const attempts = getAttempts(state.tryoutId, state.userId);
  window.localStorage.setItem(
    archiveKey(state.tryoutId, state.userId),
    JSON.stringify([...attempts, state]),
  );
}

function save(state: TryoutSessionState) {
  window.localStorage.setItem(key(state.tryoutId, state.userId), JSON.stringify(state));
}

function freshState(tryoutId: string, userId: string, attemptNumber: number): TryoutSessionState {
  const subtests = getSubtestsForTryout(tryoutId);
  return {
    tryoutId,
    userId,
    attemptNumber,
    status: "in_progress",
    startedAt: new Date().toISOString(),
    subtestStatus: Object.fromEntries(subtests.map((s) => [s.id, "pending" as SubtestStatus])),
    subtestStartedAt: {},
    answers: {},
    violationCount: 0,
    violationLog: [],
  };
}

export function initSession(tryoutId: string, userId: string): TryoutSessionState {
  const existing = loadSession(tryoutId, userId);
  if (existing) return existing;

  const state = freshState(tryoutId, userId, 1);
  save(state);
  return state;
}

export function startNewAttempt(tryoutId: string, userId: string): TryoutSessionState {
  const attemptNumber = getAttempts(tryoutId, userId).length + 1;
  const state = freshState(tryoutId, userId, attemptNumber);
  save(state);
  return state;
}

export function startSubtest(tryoutId: string, userId: string, subtestId: string): TryoutSessionState {
  const state = initSession(tryoutId, userId);
  if (state.subtestStatus[subtestId] === "pending") {
    state.subtestStatus[subtestId] = "in_progress";
    state.subtestStartedAt[subtestId] = Date.now();
    save(state);
  }
  return state;
}

export function saveAnswer(tryoutId: string, userId: string, questionId: string, answer: string[]) {
  const state = initSession(tryoutId, userId);
  state.answers[questionId] = answer;
  save(state);
  return state;
}

export function logViolation(tryoutId: string, userId: string, type: ViolationType = "exit_fullscreen") {
  const state = initSession(tryoutId, userId);
  state.violationCount += 1;
  state.violationLog.push({ type, timestamp: new Date().toISOString() });
  save(state);
  return state;
}

export function completeSubtest(tryoutId: string, userId: string, subtestId: string): TryoutSessionState {
  const state = initSession(tryoutId, userId);
  state.subtestStatus[subtestId] = "completed";

  const subtests = getSubtestsForTryout(tryoutId);
  const allDone = subtests.every((s) => state.subtestStatus[s.id] === "completed");
  if (allDone) {
    state.status = "completed";
    state.completedAt = new Date().toISOString();
    archive(state);
  }
  save(state);
  return state;
}

export function nextPendingSubtest(tryoutId: string, state: TryoutSessionState) {
  const subtests = getSubtestsForTryout(tryoutId);
  return subtests.find((s) => state.subtestStatus[s.id] !== "completed");
}

function isAnswerCorrect(question: Question, given: string[] | undefined): boolean {
  if (!given || given.length === 0) return false;
  if (question.type === "isian") {
    const normalize = (s: string) => s.trim().toLowerCase();
    return question.correctAnswer.some((ans) => normalize(ans) === normalize(given[0] ?? ""));
  }
  const correct = [...question.correctAnswer].sort();
  const g = [...given].sort();
  return correct.length === g.length && correct.every((c, i) => c === g[i]);
}

// Weighted-options questions have no wrong answer — score is the chosen option's
// points (missing points default to 1). "Correct" here just means "picked the
// highest-value option", used only for the correct/total display stat.
function weightedOptionScore(question: Question, given: string[] | undefined) {
  const options = question.options ?? [];
  const pointsOf = (id: string) => options.find((o) => o.id === id)?.points ?? 1;
  const max = options.length === 0 ? 1 : Math.max(...options.map((o) => o.points ?? 1));
  const selectedId = given?.[0];
  const earned = selectedId ? pointsOf(selectedId) : 0;
  return { earned, max, isCorrect: selectedId !== undefined && earned === max };
}

export function computeResult(
  tryout: Tryout,
  state: TryoutSessionState,
  rankContext: { rank: number; participantCount: number },
): Omit<TryoutResult, "id" | "sessionId"> {
  const subtests = getSubtestsForTryout(tryout.id);
  let earned = 0;
  let max = 0;
  const subtestScores: SubtestScore[] = subtests.map((subtest) => {
    const questions = subtest.questionIds.map((id) => getQuestion(id)).filter((q): q is Question => !!q);
    let subtestEarned = 0;
    let subtestMax = 0;
    let correctCount = 0;
    for (const q of questions) {
      if (q.optionWeighted) {
        const { earned: e, max: m, isCorrect } = weightedOptionScore(q, state.answers[q.id]);
        subtestEarned += e;
        subtestMax += m;
        if (isCorrect) correctCount += 1;
      } else {
        subtestMax += q.weight;
        if (isAnswerCorrect(q, state.answers[q.id])) {
          subtestEarned += q.weight;
          correctCount += 1;
        }
      }
    }
    earned += subtestEarned;
    max += subtestMax;
    return {
      subtestId: subtest.id,
      correct: correctCount,
      total: questions.length,
      score: subtestMax === 0 ? 0 : Math.round((subtestEarned / subtestMax) * 100),
    };
  });

  const totalScore = max === 0 ? 0 : Math.round((earned / max) * 100);
  const irtWarning = tryout.scoringMode === "irt";

  return {
    userId: state.userId,
    tryoutId: tryout.id,
    attemptNumber: state.attemptNumber,
    totalScore,
    maxScore: 100,
    scoringMode: tryout.scoringMode,
    subtestScores,
    rank: rankContext.rank,
    participantCount: rankContext.participantCount,
    irtWarning,
    completedAt: state.completedAt ?? new Date().toISOString(),
  };
}

export function estimateRank(tryoutId: string, totalScore: number) {
  const tryout = getTryout(tryoutId);
  const participantCount = (tryout?.participantCount ?? 20) + 1;
  const rank = Math.max(1, Math.min(participantCount, Math.round(((100 - totalScore) / 100) * participantCount * 0.7) + 1));
  return { rank, participantCount };
}
