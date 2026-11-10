"use client";

import { User as UserIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CardKicker, Panel } from "@/components/page-blocks";
import { AvatarSection, PasswordPanel } from "@/components/profile-sections";
import { useCommonT } from "@/lib/i18n/dict/common";
import { useStudentProfileT } from "@/lib/i18n/dict/student-profile";
import { useUser } from "@/lib/session-context";

export default function StaffProfilePage() {
  const user = useUser();
  const t = useStudentProfileT();
  const commonT = useCommonT();
  const roleLabel = user.role === "admin" ? commonT.admin : commonT.operator;
  const facts = [
    { label: t.fullName, value: user.name },
    { label: t.email, value: user.email ?? "-" },
    { label: t.phone, value: user.phone ?? "-" },
    { label: t.role, value: roleLabel },
  ];

  return (
    <AppShell title={t.title} description={t.description} maxWidth="max-w-3xl" breadcrumb={[{ label: t.crumbHome, href: "/admin" }, { label: t.title }]}>
      <div className="space-y-6">
        <Panel>
          <CardKicker icon={UserIcon} className="mb-5">
            {t.accountInfo}
          </CardKicker>
          <AvatarSection />
          <dl className="mt-5 grid gap-5 border-t border-border pt-5 sm:grid-cols-2">
            {facts.map((fact) => (
              <div key={fact.label} className="min-w-0">
                <dt className="text-caption font-medium text-foreground">{fact.label}</dt>
                <dd className="mt-1 break-words text-small text-foreground">{fact.value}</dd>
              </div>
            ))}
          </dl>
          <p className="mt-5 text-label text-muted">{t.staffHint}</p>
        </Panel>
        <PasswordPanel />
      </div>
    </AppShell>
  );
}
