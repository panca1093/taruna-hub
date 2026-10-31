"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@heroui/react";
import { ShieldAlert, WifiOff } from "lucide-react";
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
  const { status, role, retry } = useSession();
  const router = useRouter();
  const t = useRequireRoleT();
  const staffOnly = !roles.includes("student");

  useEffect(() => {
    if (status === "anonymous") router.replace(staffOnly ? "/portal" : "/login");
  }, [status, staffOnly, router]);

  if (status === "unavailable") {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-background px-5">
        <EmptyState
          icon={WifiOff}
          title={t.unavailableTitle}
          description={t.unavailableDescription}
          action={
            <Button variant="primary" size="sm" onPress={retry}>
              {t.retry}
            </Button>
          }
        />
      </div>
    );
  }

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
