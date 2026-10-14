"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "P00–P01 — Cek API, database & tenant",
  subtitle:
    "Status langsung dari GET /readyz dan GET /api/v1/tenant melalui klien API hasil generasi. Tidak ada data mock di halaman ini.",
  loading: "Memuat…",
  ok: "API dan database siap",
  okStatus: (status: string) => `status: ${status}`,
  failure: "API atau database tidak dapat dijangkau",
  tenantOk: (name: string) => `Tenant: ${name}`,
  tenantSlug: (slug: string) => `slug: ${slug}`,
  tenantFailure: "Host ini tidak menunjuk tenant aktif",
  unknownFailure: "Kegagalan tidak dikenal",
};

const en: typeof id = {
  title: "P00–P01 — API, database & tenant check",
  subtitle:
    "Live status from GET /readyz and GET /api/v1/tenant through the generated API client. No mock data on this page.",
  loading: "Loading…",
  ok: "API and database ready",
  okStatus: (status: string) => `status: ${status}`,
  failure: "The API or database is unreachable",
  tenantOk: (name: string) => `Tenant: ${name}`,
  tenantSlug: (slug: string) => `slug: ${slug}`,
  tenantFailure: "This host selects no active tenant",
  unknownFailure: "Unknown failure",
};

export function useDevApiCheckT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
