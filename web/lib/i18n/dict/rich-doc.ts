"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  imageLoading: "Memuat gambar",
  imageFailed: "Gambar gagal dimuat",
  shapes: {
    kubus: "Kubus",
    balok: "Balok",
    tabung: "Tabung",
    kerucut: "Kerucut",
    bola: "Bola",
    limas: "Limas",
    prisma: "Prisma",
  },
};

const en: typeof id = {
  imageLoading: "Loading image",
  imageFailed: "Image failed to load",
  shapes: {
    kubus: "Cube",
    balok: "Cuboid",
    tabung: "Cylinder",
    kerucut: "Cone",
    bola: "Sphere",
    limas: "Pyramid",
    prisma: "Prism",
  },
};

export function useRichDocT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
