"use client";

import type {
  Enrollment,
  Order,
  PaymentMode,
  Product,
  WalletTransaction,
} from "./types";

/**
 * Runtime layer over the seeded arrays in `mock-data.ts`, persisted to localStorage
 * so a purchase survives a reload mid-demo (same approach as `tryout-session.ts`).
 * Seeded data stays immutable; readers merge both.
 */

const STORAGE_KEY = "tryout-demo-store";

interface DemoState {
  orders: Order[];
  enrollments: Enrollment[];
  walletTransactions: WalletTransaction[];
}

const empty: DemoState = { orders: [], enrollments: [], walletTransactions: [] };

function read(): DemoState {
  if (typeof window === "undefined") return empty;
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return empty;
  try {
    return { ...empty, ...(JSON.parse(raw) as Partial<DemoState>) };
  } catch {
    return empty;
  }
}

function write(state: DemoState) {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  window.dispatchEvent(new Event(STORE_EVENT));
}

/** Pages listen for this to re-read after a purchase without a full reload. */
export const STORE_EVENT = "tryout-demo-store-change";

export const runtimeOrders = (userId: string) => read().orders.filter((o) => o.userId === userId);
export const runtimeEnrollments = (userId: string) => read().enrollments.filter((e) => e.userId === userId);
export const runtimeWalletTransactions = (userId: string) =>
  read().walletTransactions.filter((w) => w.userId === userId);

/** Net effect of runtime top-ups and deductions, added to the user's seeded balance. */
export function runtimeBalanceDelta(userId: string): number {
  return runtimeWalletTransactions(userId).reduce(
    (sum, w) => sum + (w.type === "topup" ? w.amount : -w.amount),
    0,
  );
}

export interface CreateOrderInput {
  userId: string;
  product: Product;
  paymentMode: PaymentMode;
  hasProof?: boolean;
}

/**
 * Mirrors what S6 will do server-side: gateway/wallet confirm instantly and open access,
 * manual stays pending until an admin confirms. Wallet additionally writes a ledger entry.
 */
export function createOrder({ userId, product, paymentMode, hasProof }: CreateOrderInput): Order {
  const state = read();
  const now = new Date().toISOString();
  const seq = state.orders.length + 1;
  const instantConfirm = paymentMode === "gateway" || paymentMode === "wallet";

  const order: Order = {
    id: `order-rt-${Date.now()}`,
    code: `TRX-2026-${String(9000 + seq)}`,
    userId,
    productId: product.id,
    status: instantConfirm ? "confirmed" : "pending",
    paymentMode,
    paymentProofUrl: hasProof ? "/mock/payment-proof.jpg" : undefined,
    amount: product.price,
    createdAt: now,
    confirmedAt: instantConfirm ? now : undefined,
  };

  state.orders = [...state.orders, order];

  if (instantConfirm) {
    grantAccess(state, order, product, now);
    if (paymentMode === "wallet") {
      state.walletTransactions = [
        ...state.walletTransactions,
        {
          id: `wtx-rt-${Date.now()}`,
          userId,
          type: "deduction",
          amount: product.price,
          description: product.title,
          orderId: order.id,
          createdAt: now,
        },
      ];
    }
  }

  write(state);
  return order;
}

/** Admin confirming a pending manual order — same access-granting path as an instant confirm. */
export function confirmOrder(orderId: string, product: Product): void {
  const state = read();
  const order = state.orders.find((o) => o.id === orderId);
  if (!order || order.status !== "pending") return;
  const now = new Date().toISOString();
  order.status = "confirmed";
  order.confirmedAt = now;
  grantAccess(state, order, product, now);
  write(state);
}

function grantAccess(state: DemoState, order: Order, product: Product, now: string) {
  state.enrollments = [
    ...state.enrollments,
    ...product.tryoutIds.map((tryoutId, i) => ({
      id: `enroll-rt-${Date.now()}-${i}`,
      userId: order.userId,
      tryoutId,
      orderId: order.id,
      activeUntil: product.activeUntil,
      createdAt: now,
    })),
  ];
}

/** Wipes runtime purchases — handy for resetting between demo runs. */
export function resetDemoStore() {
  window.localStorage.removeItem(STORAGE_KEY);
  window.dispatchEvent(new Event(STORE_EVENT));
}
