"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
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
  const { status, role } = useSession();
  const router = useRouter();
  const t = useRequireRoleT();
  const staffOnly = !roles.includes("student");

  useEffect(() => {
    if (status === "anonymous") router.replace(staffOnly ? "/login/staff" : "/login");
  }, [status, staffOnly, router]);

  if (status !== "authenticated") return null;

  if (!role || !roles.includes(role)) {
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
