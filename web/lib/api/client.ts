import type { components, paths } from "./schema";

export type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];
export type ErrorCode = ErrorEnvelope["error"]["code"];

/** Success body of `GET /readyz` — the walking skeleton's API + database proof. */
export type Ready = paths["/readyz"]["get"]["responses"][200]["content"]["application/json"];
/** 503 body of `GET /readyz`. This endpoint does not use the error envelope. */
export type NotReady = paths["/readyz"]["get"]["responses"][503]["content"]["application/json"];
/** Success body of `GET /api/v1/tenant` — the tenant the request host resolves to. */
export type Tenant = components["schemas"]["Tenant"];
export type ApiUser = components["schemas"]["User"];
export type TokenResponse = components["schemas"]["TokenResponse"];
export type Program = components["schemas"]["Program"];
export type ProgramList = components["schemas"]["ProgramList"];
type LoginBody = components["schemas"]["LoginRequest"];
type RegisterBody = components["schemas"]["RegisterRequest"];
type CreateProgramBody = components["schemas"]["CreateProgramRequest"];
type UpdateProgramBody = components["schemas"]["UpdateProgramRequest"];

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
    credentials: "include",
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

  if (res.status === 204 || res.status === 202) return undefined as T;
  return res.json() as Promise<T>;
}

type Json = Record<string, unknown>;

const post = <T>(path: keyof paths, body?: Json, headers?: HeadersInit) =>
  request<T>(path, {
    method: "POST",
    headers: { ...(body ? { "Content-Type": "application/json" } : {}), ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });

// The write verbs below take an already typed body; only the transport is shared.
const mutate = <T>(method: string, path: keyof paths, body?: unknown): Promise<T> =>
  authed<T>(path, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

const programPath = (id: string) => `/api/v1/programs/${id}` as keyof paths;

// The access token lives in memory only; the refresh cookie is HttpOnly and never readable here.
let accessToken: string | null = null;
let sessionLost: (() => void) | null = null;
let refreshing: Promise<TokenResponse> | null = null;

/** Called once when a protected request can no longer be saved by a refresh. */
export function onSessionLost(handler: (() => void) | null) {
  sessionLost = handler;
}

function refresh(): Promise<TokenResponse> {
  refreshing ??= post<TokenResponse>("/api/v1/auth/refresh")
    .then((tokens) => {
      accessToken = tokens.access_token;
      return tokens;
    })
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

function opened(tokens: TokenResponse): TokenResponse {
  accessToken = tokens.access_token;
  return tokens;
}

/** Sends the bearer token; on one 401 it refreshes once and retries once, then gives the session up. */
async function authed<T>(path: keyof paths, init: RequestInit = {}): Promise<T> {
  const send = () =>
    request<T>(path, { ...init, headers: { ...init.headers, Authorization: `Bearer ${accessToken}` } });
  const used = accessToken;
  if (used !== null) {
    try {
      return await send();
    } catch (err) {
      if (!(err instanceof ApiError) || err.status !== 401) throw err;
    }
  }
  try {
    // A concurrent caller may already have rotated the token this request was sent with.
    if (accessToken === used) await refresh();
    return await send();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      accessToken = null;
      sessionLost?.();
    }
    throw err;
  }
}

function envelopeError(body: unknown): ErrorEnvelope["error"] | undefined {
  if (typeof body !== "object" || body === null || !("error" in body)) return undefined;
  const { error } = body as ErrorEnvelope;
  return error && typeof error.message === "string" ? error : undefined;
}

function fallbackError(status: number): ErrorEnvelope["error"] {
  return { code: "internal_error", message: `Request failed (${status})` };
}

/** The server's own message when it sent one, otherwise the caller's fallback (network, proxy). */
export function errorText(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback;
}

export const api = {
  ready: (): Promise<Ready> =>
    request<Ready>("/readyz", undefined, (status, body) => {
      if (status !== 503) return undefined;
      const reason = (body as Partial<NotReady> | null)?.reason;
      return reason ? new ServiceUnavailableError(reason) : undefined;
    }),
  tenant: (): Promise<Tenant> => request<Tenant>("/api/v1/tenant"),

  login: (body: LoginBody) => post<TokenResponse>("/api/v1/auth/login", body).then(opened),
  register: (body: RegisterBody) => post<TokenResponse>("/api/v1/auth/register", body).then(opened),
  refresh,
  logout: async () => {
    await post<void>("/api/v1/auth/logout");
    accessToken = null;
  },
  me: () => authed<ApiUser>("/api/v1/me"),
  forgotPassword: (email: string) => post<void>("/api/v1/auth/password/forgot", { email }),
  resetPassword: (token: string, password: string) =>
    post<void>("/api/v1/auth/password/reset", { token, password }),
  verifyEmail: (code: string) =>
    authed<void>("/api/v1/me/email/verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code }),
    }),
  resendEmailCode: () => authed<void>("/api/v1/me/email/resend", { method: "POST" }),

  programs: {
    list: () => authed<ProgramList>("/api/v1/programs"),
    create: (body: CreateProgramBody) => mutate<Program>("POST", "/api/v1/programs", body),
    update: (id: string, body: UpdateProgramBody) => mutate<Program>("PUT", programPath(id), body),
    remove: (id: string) => mutate<void>("DELETE", programPath(id)),
  },
};
