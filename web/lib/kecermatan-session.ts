"use client";

import { useEffect, useState } from "react";
import { ApiError } from "./api/client";

/** 409 `kecermatan_no_credit` on start: the student has no attempt left and is sent to the purchase. */
export const isNoCredit = (err: unknown) => err instanceof ApiError && err.code === "kecermatan_no_credit";

/** Where "Beli Attempt Tambahan" goes: the product on sale for the test, else the catalog. */
export const purchaseHref = (productId: string | null | undefined) => (productId ? `/catalog/${productId}` : "/catalog");

/**
 * Whole seconds left until a server instant, counted on the server-anchored clock (`useServerClock`), so a wrong
 * device clock or a closed tab never moves a deadline. `null` until the clock has synced.
 */
export function useSecondsLeft(now: () => number | null, target: string | null | undefined) {
  const [left, setLeft] = useState<number | null>(null);
  useEffect(() => {
    if (!target) return;
    const at = new Date(target).getTime();
    const tick = () => {
      const current = now();
      if (current !== null) setLeft(Math.max(0, Math.ceil((at - current) / 1000)));
    };
    tick();
    const interval = setInterval(tick, 250);
    return () => clearInterval(interval);
  }, [now, target]);
  return target ? left : null;
}
