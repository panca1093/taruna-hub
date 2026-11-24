"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  cancel: "Batal",
  apply: "Terapkan",
  choose: "Pilih",
  search: "Cari judul…",
  noMatch: "Tidak ada yang cocok dengan pencarian.",
  tryout: "Tryout",
  kecermatan: "Tes kecermatan",
  selected: (n: number) => `${n} dipilih`,
  rule: (min: number, max: number) => `Pilih ${min} sampai ${max} item (tryout dan tes kecermatan digabung).`,
  remove: (label: string) => `Hapus ${label}`,
};

const en: typeof id = {
  cancel: "Cancel",
  apply: "Apply",
  choose: "Choose",
  search: "Search by title…",
  noMatch: "Nothing matches the search.",
  tryout: "Tryout",
  kecermatan: "Kecermatan test",
  selected: (n: number) => `${n} selected`,
  rule: (min: number, max: number) => `Choose ${min} to ${max} items (tryouts and kecermatan tests combined).`,
  remove: (label: string) => `Remove ${label}`,
};

export function useProductItemPickerT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
