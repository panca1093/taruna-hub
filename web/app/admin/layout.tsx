"use client";

import { RequireRole } from "@/components/require-role";
import { useRequireRoleT } from "@/lib/i18n/dict/require-role";

export default function AdminAreaLayout({ children }: { children: React.ReactNode }) {
  const t = useRequireRoleT();
  return (
    <RequireRole roles={["operator", "admin"]} title={t.dashboardOperatorTitle}>
      {children}
    </RequireRole>
  );
}
