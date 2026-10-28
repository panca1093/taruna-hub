"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, api } from "./api/client";

/**
 * The browser only displays what the server decided. The countdown is server time at the last sync plus the
 * monotonic time elapsed since, so changing the device clock or backgrounding the tab never moves a deadline.
 */
export function useServerClock() {
  const anchor = useRef<{ server: number; mono: number } | null>(null);

  const sync = useCallback((serverTime: string) => {
    anchor.current = { server: new Date(serverTime).getTime(), mono: performance.now() };
  }, []);

  const now = useCallback((): number | null => {
    const a = anchor.current;
    return a ? a.server + (performance.now() - a.mono) : null;
  }, []);

  return { sync, now };
}

export type AnswerState = "saved" | "pending";

interface Entry {
  value: string[];
  seq: number;
  acked: number;
  inFlight: boolean;
  closed: boolean;
}

const RETRY_MS = 3000;
const unsentKey = (sessionId: string) => `tryout-unsent:${sessionId}`;

function readUnsent(sessionId: string): Record<string, { seq: number; value: string[] }> {
  try {
    return JSON.parse(window.localStorage.getItem(unsentKey(sessionId)) ?? "{}");
  } catch {
    return {};
  }
}

function writeUnsent(sessionId: string, pending: Record<string, { seq: number; value: string[] }>) {
  try {
    if (Object.keys(pending).length === 0) window.localStorage.removeItem(unsentKey(sessionId));
    else window.localStorage.setItem(unsentKey(sessionId), JSON.stringify(pending));
  } catch {
    // Storage is a convenience; the server stays the only place an answer counts as saved.
  }
}

export interface InitialAnswer {
  id: string;
  answer: string[];
  seq: number;
}

/**
 * Sends every answer the moment it changes and tracks which ones the server has accepted. An answer written to
 * localStorage while offline is re-sent after a reload but is shown as pending until the server acknowledges it.
 * `onClosed` fires when the server says the subtest is no longer accepting answers. `loadAnswer` reads the stored
 * answer of one question, which is what a `stale_answer` rejection adopts instead of claiming the local value saved.
 */
export function useAnswerSync(
  sessionId: string | null,
  initial: InitialAnswer[] | null,
  onClosed: () => void,
  loadAnswer: (questionId: string) => Promise<InitialAnswer | null>,
) {
  const entries = useRef<Map<string, Entry>>(new Map());
  const timers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const onClosedRef = useRef(onClosed);
  const loadAnswerRef = useRef(loadAnswer);
  const [answers, setAnswers] = useState<Record<string, string[]>>({});
  const [states, setStates] = useState<Record<string, AnswerState>>({});

  useEffect(() => {
    onClosedRef.current = onClosed;
    loadAnswerRef.current = loadAnswer;
  });

  const setState = useCallback((id: string, state: AnswerState) => {
    setStates((current) => (current[id] === state ? current : { ...current, [id]: state }));
  }, []);

  const persist = useCallback(() => {
    if (!sessionId) return;
    const pending: Record<string, { seq: number; value: string[] }> = {};
    entries.current.forEach((entry, id) => {
      if (entry.seq > entry.acked) pending[id] = { seq: entry.seq, value: entry.value };
    });
    writeUnsent(sessionId, pending);
  }, [sessionId]);

  const flush = useCallback(
    function send(id: string) {
      const entry = entries.current.get(id);
      if (!entry || !sessionId || entry.inFlight || entry.closed || entry.seq <= entry.acked) return;
      const { seq, value } = entry;
      entry.inFlight = true;
      api.runtime
        .saveAnswer(sessionId, id, { seq, answer: value })
        .then(() => {
          entry.inFlight = false;
          entry.acked = Math.max(entry.acked, seq);
          if (entry.seq > entry.acked) send(id);
          else setState(id, "saved");
          persist();
        })
        .catch((err: unknown) => {
          entry.inFlight = false;
          if (err instanceof ApiError && err.status === 409) {
            if (err.code === "stale_answer") {
              const retryLater = () =>
                timers.current.set(
                  id,
                  setTimeout(() => {
                    timers.current.delete(id);
                    send(id);
                  }, RETRY_MS),
                );
              loadAnswerRef
                .current(id)
                .then((stored) => {
                  if (!stored) return retryLater();
                  if (entry.seq > seq && entry.seq > stored.seq) {
                    entry.acked = stored.seq;
                    send(id);
                    return;
                  }
                  entry.value = stored.answer;
                  entry.seq = stored.seq;
                  entry.acked = stored.seq;
                  setAnswers((current) => ({ ...current, [id]: stored.answer }));
                  setState(id, "saved");
                  persist();
                })
                .catch(retryLater);
              return;
            }
            entry.closed = true;
            onClosedRef.current();
            return;
          }
          if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 401 && err.status !== 429) {
            entry.closed = true;
            return;
          }
          timers.current.set(
            id,
            setTimeout(() => {
              timers.current.delete(id);
              send(id);
            }, RETRY_MS),
          );
        });
    },
    [sessionId, persist, setState],
  );

  useEffect(() => {
    if (!sessionId || initial === null) return;
    const map = new Map<string, Entry>();
    const values: Record<string, string[]> = {};
    const unsent = readUnsent(sessionId);
    const nextStates: Record<string, AnswerState> = {};
    for (const item of initial) {
      const local = unsent[item.id];
      const resend = local && local.seq > item.seq;
      map.set(item.id, {
        value: resend ? local.value : item.answer,
        seq: resend ? local.seq : item.seq,
        acked: item.seq,
        inFlight: false,
        closed: false,
      });
      values[item.id] = resend ? local.value : item.answer;
      nextStates[item.id] = resend ? "pending" : "saved";
    }
    entries.current = map;
    setAnswers(values);
    setStates(nextStates);
    persist();
    map.forEach((entry, id) => {
      if (entry.seq > entry.acked) flush(id);
    });
    const pendingTimers = timers.current;
    return () => {
      pendingTimers.forEach((timer) => clearTimeout(timer));
      pendingTimers.clear();
    };
  }, [sessionId, initial, flush, persist]);

  const setAnswer = useCallback(
    (id: string, value: string[]) => {
      const entry = entries.current.get(id);
      if (!entry || entry.closed) return;
      entry.seq += 1;
      entry.value = value;
      setAnswers((current) => ({ ...current, [id]: value }));
      setState(id, "pending");
      persist();
      flush(id);
    },
    [flush, persist, setState],
  );

  const waitIdle = useCallback(async (timeoutMs: number) => {
    const until = Date.now() + timeoutMs;
    const busy = () => [...entries.current.values()].some((e) => !e.closed && e.seq > e.acked);
    while (busy() && Date.now() < until) await new Promise((resolve) => setTimeout(resolve, 150));
  }, []);

  const pendingCount = Object.values(states).filter((s) => s === "pending").length;

  return { answers, states, setAnswer, pendingCount, waitIdle };
}
