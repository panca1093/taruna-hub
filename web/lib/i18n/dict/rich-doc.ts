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
    persegi: "Persegi",
    persegi_panjang: "Persegi panjang",
    segitiga: "Segitiga",
    lingkaran: "Lingkaran",
    trapesium: "Trapesium",
    jajar_genjang: "Jajar genjang",
    belah_ketupat: "Belah ketupat",
    layang_layang: "Layang-layang",
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
    persegi: "Square",
    persegi_panjang: "Rectangle",
    segitiga: "Triangle",
    lingkaran: "Circle",
    trapesium: "Trapezoid",
    jajar_genjang: "Parallelogram",
    belah_ketupat: "Rhombus",
    layang_layang: "Kite",
  },
};

export function useRichDocT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
