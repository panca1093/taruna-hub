"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  dashboardOperatorTitle: "Dashboard Operator",
  dashboardStudentTitle: "Dashboard Siswa",
  accessRestrictedTitle: "Akses terbatas",
  accessRestrictedDescription:
    "Akun kamu tidak punya akses ke halaman ini.",
  unavailableTitle: "Server tidak dapat dihubungi",
  unavailableDescription: "Sesi kamu tidak terpengaruh. Coba muat ulang sebentar lagi.",
  retry: "Coba lagi",
};

const en: typeof id = {
  dashboardOperatorTitle: "Operator Dashboard",
  dashboardStudentTitle: "Student Dashboard",
  accessRestrictedTitle: "Access restricted",
  accessRestrictedDescription:
    "Your account does not have access to this page.",
  unavailableTitle: "Cannot reach the server",
  unavailableDescription: "Your session is not affected. Try again in a moment.",
  retry: "Try again",
};

export function useRequireRoleT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
