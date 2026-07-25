"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  dashboardOperatorTitle: "Dashboard Operator",
  accessRestrictedTitle: "Akses terbatas",
  accessRestrictedDescription:
    "Halaman ini hanya untuk tim internal. Ganti tampilan ke Operator atau Admin dari menu akun di sidebar.",
};

const en: typeof id = {
  dashboardOperatorTitle: "Operator Dashboard",
  accessRestrictedTitle: "Access restricted",
  accessRestrictedDescription:
    "This page is for internal team only. Switch to Operator or Admin view from the account menu in the sidebar.",
};

export function useRequireRoleT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
