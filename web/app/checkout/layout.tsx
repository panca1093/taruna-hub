"use client";

import { RequireRole } from "@/components/require-role";
import { useRequireRoleT } from "@/lib/i18n/dict/require-role";

export default function CheckoutAreaLayout({ children }: { children: React.ReactNode }) {
  const t = useRequireRoleT();
  return (
    <RequireRole roles={["student"]} title={t.dashboardStudentTitle}>
      {children}
    </RequireRole>
  );
}
