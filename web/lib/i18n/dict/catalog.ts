"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  title: "Katalog",
  description: "Paket individual berisi satu tryout. Paket bundle berisi beberapa tryout dengan harga lebih hemat.",
  publicEyebrow: "Katalog",
  publicHeading: "Pilih paket tryout sesuai jenjangmu",
  filterAll: "Semua",
  emptyTitle: "Belum ada paket untuk jenjang ini",
  emptyDescription: "Coba pilih jenjang lain atau kembali lagi nanti.",
  sectionAcademic: "Tes Potensi Akademik",
  sectionPsychometric: "Psikotes",
  typeIndividual: "Individual",
  typeKecermatan: "Kecermatan",
  typeBundle: (n: number) => `Bundle · ${n}x`,
};

const en: typeof id = {
  title: "Catalog",
  description: "Individual packages include one tryout. Bundle packages include several tryouts at a lower price.",
  publicEyebrow: "Catalog",
  publicHeading: "Choose the right package for your level",
  filterAll: "All",
  emptyTitle: "No packages for this level yet",
  emptyDescription: "Try another level or check back later.",
  sectionAcademic: "Academic Potential Test",
  sectionPsychometric: "Psychometric Test",
  typeIndividual: "Individual",
  typeKecermatan: "Kecermatan",
  typeBundle: (n: number) => `Bundle · ${n}x`,
};

export function useCatalogT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
