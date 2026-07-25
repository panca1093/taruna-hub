"use client";

import { ShieldAlert } from "lucide-react";
import { AppShell } from "./app-shell";
import { EmptyState } from "./empty-state";
import { useSession } from "@/lib/session-context";
import { useRequireRoleT } from "@/lib/i18n/dict/require-role";
import type { Role } from "@/lib/types";

export function RequireRole({
  roles,
  title,
  deniedMessage,
  children,
}: {
  roles: Role[];
  title: string;
  deniedMessage?: string;
  children: React.ReactNode;
}) {
  const { role: current } = useSession();
  const t = useRequireRoleT();

  if (!roles.includes(current)) {
    return (
      <AppShell title={title}>
        <EmptyState
          icon={ShieldAlert}
          title={t.accessRestrictedTitle}
          description={deniedMessage ?? t.accessRestrictedDescription}
        />
      </AppShell>
    );
  }

  return <>{children}</>;
}
