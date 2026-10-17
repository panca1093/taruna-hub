"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { ApiError, api, onSessionLost, type ApiUser, type TokenResponse } from "./api/client";
import type { Role } from "./types";

export interface SessionUser {
  id: string;
  name: string;
  email: string | null;
  phone: string | null;
  /** Null for a role this app has no screens for (platform_admin). */
  role: Role | null;
  emailVerified: boolean;
}

// "unavailable": the API could not be reached, which says nothing about the session.
type Status = "loading" | "anonymous" | "authenticated" | "unavailable";

interface SessionContextValue {
  status: Status;
  user: SessionUser | null;
  role: Role | null;
  login: (identifier: string, password: string) => Promise<SessionUser>;
  register: (body: { name: string; email: string; phone?: string; password: string }) => Promise<SessionUser>;
  logout: () => Promise<void>;
  retry: () => void;
  reloadUser: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

const appRoles: readonly string[] = ["student", "operator", "admin"];
// The server moves the idle deadline on any authenticated request, at most once a minute.
const ACTIVITY_INTERVAL_MS = 60_000;

export const homeFor = (role: Role | null) => (role === "student" ? "/student" : role ? "/admin" : "/login");

function toUser(u: ApiUser): SessionUser {
  return {
    id: u.id,
    name: u.name,
    email: u.email,
    phone: u.phone,
    role: appRoles.includes(u.role) ? (u.role as Role) : null,
    emailVerified: u.email_verified,
  };
}

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<{ status: Status; user: SessionUser | null }>({
    status: "loading",
    user: null,
  });
  const lastActivity = useRef(0);
  // Bumped by anything that decides the session, so a slower boot refresh cannot overwrite it.
  const epoch = useRef(0);
  const pathname = usePathname();

  const open = useCallback((tokens: TokenResponse) => {
    const user = toUser(tokens.user);
    epoch.current++;
    lastActivity.current = Date.now();
    setState({ status: "authenticated", user });
    return user;
  }, []);

  const boot = useCallback(() => {
    const mine = epoch.current;
    api.refresh().then(
      (tokens) => {
        if (epoch.current === mine) setState({ status: "authenticated", user: toUser(tokens.user) });
      },
      (err) => {
        if (epoch.current !== mine) return;
        const rejected = err instanceof ApiError && (err.status === 401 || err.status === 403);
        setState({ status: rejected ? "anonymous" : "unavailable", user: null });
      },
    );
  }, []);

  const retry = useCallback(() => {
    setState({ status: "loading", user: null });
    boot();
  }, [boot]);

  useEffect(() => {
    boot();
    onSessionLost(() => {
      epoch.current++;
      setState({ status: "anonymous", user: null });
    });
    return () => onSessionLost(null);
  }, [boot]);

  const authenticated = state.status === "authenticated";
  useEffect(() => {
    if (!authenticated || Date.now() - lastActivity.current < ACTIVITY_INTERVAL_MS) return;
    lastActivity.current = Date.now();
    api.me().then(
      (u) => setState({ status: "authenticated", user: toUser(u) }),
      () => {},
    );
  }, [authenticated, pathname]);

  const login = useCallback(
    async (identifier: string, password: string) => open(await api.login({ identifier, password })),
    [open],
  );
  const register = useCallback<SessionContextValue["register"]>(
    async (body) => open(await api.register(body)),
    [open],
  );
  const logout = useCallback(async () => {
    await api.logout();
    epoch.current++;
    setState({ status: "anonymous", user: null });
  }, []);
  const reloadUser = useCallback(async () => {
    const u = await api.me();
    setState({ status: "authenticated", user: toUser(u) });
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({ ...state, role: state.user?.role ?? null, login, register, logout, retry, reloadUser }),
    [state, login, register, logout, retry, reloadUser],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession() {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used within SessionProvider");
  return ctx;
}

/** For screens that sit behind RequireRole, where a user is guaranteed. */
export function useUser(): SessionUser {
  const { user } = useSession();
  if (!user) throw new Error("useUser needs an authenticated session");
  return user;
}
