import type { components, paths } from "./schema";

export type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];
export type ErrorCode = ErrorEnvelope["error"]["code"];

/** Success body of `GET /readyz` — the walking skeleton's API + database proof. */
export type Ready = paths["/readyz"]["get"]["responses"][200]["content"]["application/json"];
/** 503 body of `GET /readyz`. This endpoint does not use the error envelope. */
export type NotReady = paths["/readyz"]["get"]["responses"][503]["content"]["application/json"];
/** Success body of `GET /api/v1/tenant` — the tenant the request host resolves to. */
export type Tenant = components["schemas"]["Tenant"];

// Production sets NEXT_PUBLIC_API_URL to "" (same-origin); unset in dev, call the API on the page's own host.
function baseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL;
  if (configured !== undefined) return configured;
  if (typeof window === "undefined") return "http://localhost:8081";
  return `${window.location.protocol}//${window.location.hostname}:8081`;
}

/** Thrown for any non-2xx response, carrying the server's stable error code. */
export class ApiError extends Error {
  readonly code: ErrorCode;
  readonly status: number;
  readonly fields?: Record<string, string>;
  readonly requestId?: string;

  constructor(status: number, body: ErrorEnvelope["error"]) {
    super(body.message);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.fields = body.fields;
    this.requestId = body.request_id;
  }
}

// /readyz answers 503 with its own {status, reason} body, not the error envelope.
export class ServiceUnavailableError extends Error {
  readonly status = 503;
  readonly reason: string;

  constructor(reason: string) {
    super(reason);
    this.name = "ServiceUnavailableError";
    this.reason = reason;
  }
}

async function request<T>(
  path: keyof paths,
  init?: RequestInit,
  mapError?: (status: number, body: unknown) => Error | undefined,
): Promise<T> {
  const res = await fetch(`${baseUrl()}${path}`, {
    ...init,
    headers: { Accept: "application/json", ...init?.headers },
  });

  if (!res.ok) {
    // A non-JSON body here means the failure happened before the API could shape a
    // response (proxy, gateway, network); surface it as internal_error rather than crashing.
    const body: unknown = await res.json().catch(() => null);
    throw (
      mapError?.(res.status, body) ??
      new ApiError(res.status, envelopeError(body) ?? fallbackError(res.status))
    );
  }

  return res.json() as Promise<T>;
}

function envelopeError(body: unknown): ErrorEnvelope["error"] | undefined {
  if (typeof body !== "object" || body === null || !("error" in body)) return undefined;
  const { error } = body as ErrorEnvelope;
  return error && typeof error.message === "string" ? error : undefined;
}

function fallbackError(status: number): ErrorEnvelope["error"] {
  return { code: "internal_error", message: `Request failed (${status})` };
}

export const api = {
  ready: (): Promise<Ready> =>
    request<Ready>("/readyz", undefined, (status, body) => {
      if (status !== 503) return undefined;
      const reason = (body as Partial<NotReady> | null)?.reason;
      return reason ? new ServiceUnavailableError(reason) : undefined;
    }),
  tenant: (): Promise<Tenant> => request<Tenant>("/api/v1/tenant"),
};
