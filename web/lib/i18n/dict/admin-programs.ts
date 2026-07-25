"use client";

import { useLocale } from "@/lib/i18n/locale-context";

const id = {
  requireRoleTitle: "Program",
  requireRoleDenied: "Kelola program hanya bisa dilakukan oleh Admin. Hubungi Admin bila kamu Operator dan perlu perubahan.",
  title: "Program",
  description: "Kelola jenjang / program tryout",
  addProgram: "Tambah Program",
  emptyTitle: "Belum ada program",
  columns: {
    code: "Kode",
    name: "Nama",
    description: "Deskripsi",
    subjectsHeader: "Mapel",
    subjects: (n: number) => `${n} mapel`,
    tryouts: "Tryout",
  },
  editTitle: "Edit Program",
  createTitle: "Tambah Program",
  cancel: "Batal",
  save: "Simpan",
  codeLabel: "Kode",
  codePlaceholder: "mis. SD",
  nameLabel: "Nama program",
  namePlaceholder: "mis. Sekolah Dasar",
  descriptionLabel: "Deskripsi",
  subjectsHeading: "Mata pelajaran & bab materi",
  subjectsHelp: "Menentukan pilihan mapel dan bab materi (dropdown) saat membuat soal untuk program ini.",
  subjectPlaceholder: "mis. Matematika",
  addSubject: "Tambah Mapel",
  chapterPlaceholder: "Tambah bab materi, mis. Aljabar",
  addChapter: "Bab",
  noSubjects: "Belum ada mata pelajaran ditambahkan.",
  toastUpdated: "Program diperbarui",
  toastCreated: "Program ditambahkan",
  toastCannotDeleteTitle: "Tidak bisa dihapus",
  toastCannotDeleteDescription: "Program masih digunakan oleh tryout atau produk.",
  toastDeleted: "Program dihapus",
};

const en: typeof id = {
  requireRoleTitle: "Programs",
  requireRoleDenied: "Managing programs is Admin-only. Contact an Admin if you're an Operator and need a change.",
  title: "Programs",
  description: "Manage tryout levels / programs",
  addProgram: "Add Program",
  emptyTitle: "No programs yet",
  columns: {
    code: "Code",
    name: "Name",
    description: "Description",
    subjectsHeader: "Subjects",
    subjects: (n: number) => `${n} subjects`,
    tryouts: "Tryouts",
  },
  editTitle: "Edit Program",
  createTitle: "Add Program",
  cancel: "Cancel",
  save: "Save",
  codeLabel: "Code",
  codePlaceholder: "e.g. SD",
  nameLabel: "Program name",
  namePlaceholder: "e.g. Elementary School",
  descriptionLabel: "Description",
  subjectsHeading: "Subjects & chapters",
  subjectsHelp: "Determines the subject and chapter (dropdown) choices when creating a question for this program.",
  subjectPlaceholder: "e.g. Math",
  addSubject: "Add Subject",
  chapterPlaceholder: "Add chapter, e.g. Algebra",
  addChapter: "Chapter",
  noSubjects: "No subjects added yet.",
  toastUpdated: "Program updated",
  toastCreated: "Program added",
  toastCannotDeleteTitle: "Can't delete",
  toastCannotDeleteDescription: "This program is still used by a tryout or product.",
  toastDeleted: "Program deleted",
};

export function useAdminProgramsT() {
  const { locale } = useLocale();
  return locale === "en" ? en : id;
}
