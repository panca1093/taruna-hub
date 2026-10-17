"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  orderStatus: { pending: "Menunggu", confirmed: "Terkonfirmasi", cancelled: "Dibatalkan" },
  difficulty: { easy: "Mudah", medium: "Sedang", hard: "Sulit" },
  questionType: { pg: "Pilihan Ganda", pg_multi: "PG Kompleks", isian: "Isian Singkat", benar_salah: "Benar/Salah" },
  sessionStatus: { in_progress: "Sedang mengerjakan", completed: "Selesai" },
  tryoutStatus: { draft: "Draf", published: "Terbit" },
};

const en: typeof id = {
  orderStatus: { pending: "Pending", confirmed: "Confirmed", cancelled: "Cancelled" },
  difficulty: { easy: "Easy", medium: "Medium", hard: "Hard" },
  questionType: { pg: "Multiple Choice", pg_multi: "Complex MC", isian: "Short Answer", benar_salah: "True/False" },
  sessionStatus: { in_progress: "In progress", completed: "Completed" },
  tryoutStatus: { draft: "Draft", published: "Published" },
};

export function useStatusBadgeT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
