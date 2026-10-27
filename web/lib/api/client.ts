import type { components, paths } from "./schema";

export type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];
export type ErrorCode = ErrorEnvelope["error"]["code"];

/** Success body of `GET /readyz` — the walking skeleton's API + database proof. */
export type Ready = paths["/readyz"]["get"]["responses"][200]["content"]["application/json"];
/** 503 body of `GET /readyz`. This endpoint does not use the error envelope. */
export type NotReady = paths["/readyz"]["get"]["responses"][503]["content"]["application/json"];
/** Success body of `GET /api/v1/tenant` — the tenant this deployment is configured with. */
export type Tenant = components["schemas"]["Tenant"];
export type ApiUser = components["schemas"]["User"];
export type TokenResponse = components["schemas"]["TokenResponse"];
export type Program = components["schemas"]["Program"];
export type ProgramList = components["schemas"]["ProgramList"];
export type Question = components["schemas"]["Question"];
export type QuestionList = components["schemas"]["QuestionList"];
export type QuestionOption = components["schemas"]["QuestionOption"];
export type QuestionImportPreview = components["schemas"]["QuestionImportPreview"];
export type QuestionImportRow = components["schemas"]["QuestionImportRow"];
export type QuestionImportCommit = components["schemas"]["QuestionImportCommit"];
/** The three formats the import template is generated in. */
export type QuestionTemplateFormat = "xlsx" | "csv" | "docx";
export type Product = components["schemas"]["Product"];
export type ProductList = components["schemas"]["ProductList"];
export type PublicProduct = components["schemas"]["PublicProduct"];
export type PublicProductList = components["schemas"]["PublicProductList"];
export type CatalogSummary = components["schemas"]["CatalogSummary"];
export type Wallet = components["schemas"]["Wallet"];
export type WalletEntry = components["schemas"]["WalletEntry"];
export type WalletEntryList = components["schemas"]["WalletEntryList"];
export type WalletStudent = components["schemas"]["WalletStudent"];
export type WalletStudentList = components["schemas"]["WalletStudentList"];
export type AdjustmentResult = components["schemas"]["AdjustmentResult"];
export type Tryout = components["schemas"]["Tryout"];
export type TryoutSummary = components["schemas"]["TryoutSummary"];
export type TryoutList = components["schemas"]["TryoutList"];
export type TryoutSubtest = components["schemas"]["Subtest"];
export type TryoutAssignment = components["schemas"]["Assignment"];
type LoginBody = components["schemas"]["LoginRequest"];
type RegisterBody = components["schemas"]["RegisterRequest"];
type CreateProgramBody = components["schemas"]["CreateProgramRequest"];
type UpdateProgramBody = components["schemas"]["UpdateProgramRequest"];
type CreateQuestionBody = components["schemas"]["CreateQuestionRequest"];
type UpdateQuestionBody = components["schemas"]["UpdateQuestionRequest"];
type CreateTryoutBody = components["schemas"]["CreateTryoutRequest"];
type UpdateTryoutBody = components["schemas"]["UpdateTryoutRequest"];
type ProductBody = components["schemas"]["ProductRequest"];

/** Filters of `GET /api/v1/questions`; an absent field is not sent at all. */
export type QuestionFilters = Omit<
  NonNullable<paths["/api/v1/questions"]["get"]["parameters"]["query"]>,
  "limit" | "offset"
> & { limit?: number; offset?: number };

/** Filters of `GET /api/v1/tryouts`; an absent field is not sent at all. */
export type TryoutFilters = Omit<
  NonNullable<paths["/api/v1/tryouts"]["get"]["parameters"]["query"]>,
  "limit" | "offset"
> & { limit?: number; offset?: number };

/** Filters of `GET /api/v1/products`; an absent field is not sent at all. */
export type ProductFilters = Omit<
  NonNullable<paths["/api/v1/products"]["get"]["parameters"]["query"]>,
  "limit" | "offset"
> & { limit?: number; offset?: number };

/** Filters of the public `GET /api/v1/catalog/products`; there is no status filter in public. */
export type CatalogFilters = Omit<
  NonNullable<paths["/api/v1/catalog/products"]["get"]["parameters"]["query"]>,
  "limit" | "offset"
> & { limit?: number; offset?: number };

/** Page of `GET /api/v1/wallet/entries`; the student's own ledger or one student's, by role. */
export type WalletEntryFilters = Omit<
  NonNullable<paths["/api/v1/wallet/entries"]["get"]["parameters"]["query"]>,
  "limit" | "offset"
> & { limit?: number; offset?: number };

/** Filters of `GET /api/v1/students`; an absent field is not sent at all. */
export type WalletStudentFilters = Omit<
  NonNullable<paths["/api/v1/students"]["get"]["parameters"]["query"]>,
  "limit" | "offset"
> & { limit?: number; offset?: number };

type AdjustmentBody = components["schemas"]["AdjustmentRequest"];

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

const questionPath = (id: string) => `/api/v1/questions/${id}` as keyof paths;

const tryoutPath = (id: string) => `/api/v1/tryouts/${id}` as keyof paths;

const productPath = (id: string) => `/api/v1/products/${id}` as keyof paths;

const catalogProductPath = (id: string) => `/api/v1/catalog/products/${id}` as keyof paths;

/** Query string of a list request; empty values are dropped so the server sees no filter. */
function queryOf(filters: Record<string, string | number | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== undefined && value !== "") params.set(key, String(value));
  }
  const query = params.toString();
  return query === "" ? "" : `?${query}`;
}

/** Multipart upload of a question file; `rows` is the confirmed selection of a commit. */
function uploadForm<T>(path: keyof paths, file: File, rows?: number[]): Promise<T> {
  const body = new FormData();
  body.append("file", file);
  if (rows !== undefined) body.append("rows", JSON.stringify(rows));
  return authed<T>(path, { method: "POST", body });
}

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

/** A file download: the same auth and one-refresh retry as `authed`, but the body is not JSON. */
async function authedBlob(path: keyof paths): Promise<Blob> {
  const send = () =>
    fetch(`${baseUrl()}${path}`, {
      credentials: "include",
      headers: { Accept: "*/*", Authorization: `Bearer ${accessToken}` },
    });
  const used = accessToken;
  if (used !== null) {
    const res = await send();
    if (res.status !== 401) return blobOf(res);
  }
  if (accessToken === used) await refresh();
  const res = await send();
  if (res.status === 401) {
    accessToken = null;
    sessionLost?.();
  }
  return blobOf(res);
}

async function blobOf(res: Response): Promise<Blob> {
  if (!res.ok) {
    const body: unknown = await res.json().catch(() => null);
    throw new ApiError(res.status, envelopeError(body) ?? fallbackError(res.status));
  }
  return res.blob();
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

  questions: {
    list: (filters: QuestionFilters = {}) =>
      authed<QuestionList>(`/api/v1/questions${queryOf(filters)}` as keyof paths),
    get: (id: string) => authed<Question>(questionPath(id)),
    create: (body: CreateQuestionBody) => mutate<Question>("POST", "/api/v1/questions", body),
    update: (id: string, body: UpdateQuestionBody) => mutate<Question>("PUT", questionPath(id), body),
    remove: (id: string) => mutate<void>("DELETE", questionPath(id)),
    importPreview: (file: File) =>
      uploadForm<QuestionImportPreview>("/api/v1/questions/import/preview", file),
    importCommit: (file: File, rows: number[]) =>
      uploadForm<QuestionImportCommit>("/api/v1/questions/import/commit", file, rows),
    importTemplate: (format: QuestionTemplateFormat) =>
      authedBlob(`/api/v1/questions/import/template?format=${format}` as keyof paths),
  },

  products: {
    list: (filters: ProductFilters = {}) =>
      authed<ProductList>(`/api/v1/products${queryOf(filters)}` as keyof paths),
    get: (id: string) => authed<Product>(productPath(id)),
    create: (body: ProductBody) => mutate<Product>("POST", "/api/v1/products", body),
    update: (id: string, body: ProductBody) => mutate<Product>("PUT", productPath(id), body),
    remove: (id: string) => mutate<void>("DELETE", productPath(id)),
  },

  /** The public catalog: no Authorization header, so it works for an anonymous visitor. */
  catalog: {
    products: (filters: CatalogFilters = {}) =>
      request<PublicProductList>(`/api/v1/catalog/products${queryOf(filters)}` as keyof paths),
    product: (id: string) => request<PublicProduct>(catalogProductPath(id)),
    summary: () => request<CatalogSummary>("/api/v1/catalog/summary"),
  },

  /** The signed-in student's own wallet and ledger; staff routes live under `students`. */
  wallet: {
    get: () => authed<Wallet>("/api/v1/wallet"),
    entries: (filters: WalletEntryFilters = {}) =>
      authed<WalletEntryList>(`/api/v1/wallet/entries${queryOf(filters)}` as keyof paths),
  },

  students: {
    list: (filters: WalletStudentFilters = {}) =>
      authed<WalletStudentList>(`/api/v1/students${queryOf(filters)}` as keyof paths),
    wallet: (id: string) => authed<Wallet>(`/api/v1/students/${id}/wallet` as keyof paths),
    entries: (id: string, filters: WalletEntryFilters = {}) =>
      authed<WalletEntryList>(`/api/v1/students/${id}/wallet/entries${queryOf(filters)}` as keyof paths),
    adjust: (id: string, body: AdjustmentBody) =>
      mutate<AdjustmentResult>("POST", `/api/v1/students/${id}/wallet/adjustments` as keyof paths, body),
  },

  tryouts: {
    list: (filters: TryoutFilters = {}) =>
      authed<TryoutList>(`/api/v1/tryouts${queryOf(filters)}` as keyof paths),
    get: (id: string) => authed<Tryout>(tryoutPath(id)),
    create: (body: CreateTryoutBody) => mutate<Tryout>("POST", "/api/v1/tryouts", body),
    update: (id: string, body: UpdateTryoutBody) => mutate<Tryout>("PUT", tryoutPath(id), body),
    remove: (id: string) => mutate<void>("DELETE", tryoutPath(id)),
    publish: (id: string) => mutate<Tryout>("POST", `${tryoutPath(id)}/publish` as keyof paths),
    unpublish: (id: string) => mutate<Tryout>("POST", `${tryoutPath(id)}/unpublish` as keyof paths),
  },
};
