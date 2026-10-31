import type { Role } from "./types";

const STUDENT_AREAS = ["/student", "/checkout", "/catalog"];

const inArea = (pathname: string, area: string) => pathname === area || pathname.startsWith(`${area}/`);

/**
 * Returns a same-origin path a signed-in user may be sent back to, or null.
 * Only students carry return intent; staff always land on their own home.
 */
export function safeReturnPath(raw: string | null | undefined, role: Role | null): string | null {
  if (role !== "student" || !raw || raw.length > 512) return null;
  if (!raw.startsWith("/") || raw.startsWith("//") || /[\\\u0000-\u001f\u007f]/.test(raw)) return null;
  let url: URL;
  try {
    url = new URL(raw, "http://return.invalid");
  } catch {
    return null;
  }
  if (url.origin !== "http://return.invalid") return null;
  // Checked on the normalised pathname so `/student/../portal` cannot slip through.
  if (!STUDENT_AREAS.some((area) => inArea(url.pathname, area))) return null;
  return url.pathname + url.search;
}

/** Appends return intent to an auth route; callers pass a path already taken from the current location. */
export function withReturn(route: string, next: string | null | undefined): string {
  return next ? `${route}?next=${encodeURIComponent(next)}` : route;
}
