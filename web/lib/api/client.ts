import type { components, paths } from "./schema";

export type User = components["schemas"]["User"];
export type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];
export type ErrorCode = ErrorEnvelope["error"]["code"];

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8081";

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

async function request<T>(path: keyof paths, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: { Accept: "application/json", ...init?.headers },
  });

  if (!res.ok) {
    // A non-JSON body here means the failure happened before the API could shape a
    // response (proxy, gateway, network); surface it as internal_error rather than crashing.
    const body = await res.json().catch(() => null);
    throw new ApiError(
      res.status,
      body?.error ?? { code: "internal_error", message: `Request failed (${res.status})` },
    );
  }

  return res.json() as Promise<T>;
}

export const api = {
  me: (demoUser?: string) =>
    request<User>("/api/v1/me", {
      headers: demoUser ? { "X-Demo-User": demoUser } : undefined,
    }),
};
