"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { AppUser, Role } from "./types";
import { currentAdmin, currentOperator, currentStudent } from "./mock-data";

const usersByRole: Record<Role, AppUser> = {
  student: currentStudent,
  operator: currentOperator,
  admin: currentAdmin,
};

interface SessionContextValue {
  role: Role;
  user: AppUser;
  setRole: (role: Role) => void;
}

const SessionContext = createContext<SessionContextValue | null>(null);

const STORAGE_KEY = "tryout-demo-role";
const SESSION_SETTINGS_KEY = "tryout-session-settings";

export interface SessionSettings {
  autoLogout: boolean;
  idleMinutes: number;
}

const defaultSessionSettings: SessionSettings = { autoLogout: true, idleMinutes: 30 };

export function getSessionSettings(): SessionSettings {
  if (typeof window === "undefined") return defaultSessionSettings;
  try {
    const raw = window.localStorage.getItem(SESSION_SETTINGS_KEY);
    return raw ? { ...defaultSessionSettings, ...JSON.parse(raw) } : defaultSessionSettings;
  } catch {
    return defaultSessionSettings;
  }
}

export function saveSessionSettings(settings: SessionSettings) {
  window.localStorage.setItem(SESSION_SETTINGS_KEY, JSON.stringify(settings));
}

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [role, setRoleState] = useState<Role>("student");

  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY) as Role | null;
    if (stored && usersByRole[stored]) setRoleState(stored);
  }, []);

  const setRole = useCallback((next: Role) => {
    setRoleState(next);
    window.localStorage.setItem(STORAGE_KEY, next);
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({ role, user: usersByRole[role], setRole }),
    [role, setRole],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession() {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used within SessionProvider");
  return ctx;
}
