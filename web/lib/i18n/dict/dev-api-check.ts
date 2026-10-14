"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "P00 — Cek API & database",
  subtitle:
    "Status langsung dari GET /readyz melalui klien API hasil generasi. Tidak ada data mock di halaman ini.",
  loading: "Memuat…",
  ok: "API dan database siap",
  okStatus: (status: string) => `status: ${status}`,
  failure: "API atau database tidak dapat dijangkau",
  unknownFailure: "Kegagalan tidak dikenal",
};

const en: typeof id = {
  title: "P00 — API & database check",
  subtitle:
    "Live status from GET /readyz through the generated API client. No mock data on this page.",
  loading: "Loading…",
  ok: "API and database ready",
  okStatus: (status: string) => `status: ${status}`,
  failure: "The API or database is unreachable",
  unknownFailure: "Unknown failure",
};

export function useDevApiCheckT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
