/** Formats a canonical decimal money string for display only; it never re-computes the amount. */
export function formatIDR(value: string): string {
  const [whole, fraction] = value.split(".");
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  if (fraction === undefined || fraction === "00") return `Rp${grouped}`;
  return `Rp${grouped},${fraction}`;
}

/** A signed ledger amount: the sign is shown, the magnitude is formatted, nothing is recomputed. */
export function formatSignedIDR(value: string): string {
  const negative = value.startsWith("-");
  return `${negative ? "−" : "+"}${formatIDR(negative ? value.slice(1) : value)}`;
}
