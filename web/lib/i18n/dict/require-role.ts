"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  dashboardOperatorTitle: "Dashboard Operator",
  dashboardStudentTitle: "Dashboard Siswa",
  accessRestrictedTitle: "Akses terbatas",
  accessRestrictedDescription:
    "Akun kamu tidak punya akses ke halaman ini.",
};

const en: typeof id = {
  dashboardOperatorTitle: "Operator Dashboard",
  dashboardStudentTitle: "Student Dashboard",
  accessRestrictedTitle: "Access restricted",
  accessRestrictedDescription:
    "Your account does not have access to this page.",
};

export function useRequireRoleT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
