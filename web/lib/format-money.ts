/** Formats a canonical decimal money string for display only; it never re-computes the amount. */
export function formatIDR(value: string): string {
  const [whole, fraction] = value.split(".");
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  if (fraction === undefined || fraction === "00") return `Rp${grouped}`;
  return `Rp${grouped},${fraction}`;
}

// Canonical money has at most 13 digits, so integer cents are exact in a double; this never rounds.
const centsOf = (value: string): number => {
  const [whole, fraction = ""] = value.split(".");
  return Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
};

/** Display-only comparison, so checkout can warn before the server refuses; the server decides. */
export function isLessMoney(value: string, other: string): boolean {
  return centsOf(value) < centsOf(other);
}

/** Display-only `value - other` of two canonical non-negative strings, for "remaining balance" style lines. */
export function subtractMoney(value: string, other: string): string {
  const cents = centsOf(value) - centsOf(other);
  const sign = cents < 0 ? "-" : "";
  const abs = Math.abs(cents);
  return `${sign}${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, "0")}`;
}

/** A signed ledger amount: the sign is shown, the magnitude is formatted, nothing is recomputed. */
export function formatSignedIDR(value: string): string {
  const negative = value.startsWith("-");
  return `${negative ? "−" : "+"}${formatIDR(negative ? value.slice(1) : value)}`;
}
