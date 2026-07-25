"use client";

import { useState } from "react";
import { Button, toast } from "@heroui/react";
import { Pencil, Plus, School, Trash2, X } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput, TextareaInput } from "@/components/ui/form";
import { programs as initialPrograms, products, tryouts } from "@/lib/mock-data";
import type { ProgramSubject } from "@/lib/types";
import { useAdminProgramsT } from "@/lib/i18n/dict/admin-programs";

interface ProgramRow {
  id: string;
  code: string;
  name: string;
  description: string;
  subjects: ProgramSubject[];
}

export default function ProgramsPage() {
  const t = useAdminProgramsT();
  const [rows, setRows] = useState<ProgramRow[]>(initialPrograms);
  const [editing, setEditing] = useState<ProgramRow | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [form, setForm] = useState({ code: "", name: "", description: "", subjects: [] as ProgramSubject[] });
  const [subjectInput, setSubjectInput] = useState("");
  const [chapterInputBySubject, setChapterInputBySubject] = useState<Record<string, string>>({});

  const openCreate = () => {
    setEditing(null);
    setForm({ code: "", name: "", description: "", subjects: [] });
    setSubjectInput("");
    setModalOpen(true);
  };

  const openEdit = (row: ProgramRow) => {
    setEditing(row);
    setForm({ code: row.code, name: row.name, description: row.description, subjects: row.subjects });
    setSubjectInput("");
    setModalOpen(true);
  };

  const addSubject = () => {
    const value = subjectInput.trim();
    if (!value || form.subjects.some((s) => s.name.toLowerCase() === value.toLowerCase())) {
      setSubjectInput("");
      return;
    }
    setForm((f) => ({ ...f, subjects: [...f.subjects, { id: `subj-${Date.now()}`, name: value, chapters: [] }] }));
    setSubjectInput("");
  };

  const removeSubject = (subjectId: string) => {
    setForm((f) => ({ ...f, subjects: f.subjects.filter((s) => s.id !== subjectId) }));
  };

  const addChapter = (subjectId: string) => {
    const value = (chapterInputBySubject[subjectId] ?? "").trim();
    if (!value) return;
    setForm((f) => ({
      ...f,
      subjects: f.subjects.map((s) =>
        s.id === subjectId && !s.chapters.some((c) => c.toLowerCase() === value.toLowerCase())
          ? { ...s, chapters: [...s.chapters, value] }
          : s,
      ),
    }));
    setChapterInputBySubject((m) => ({ ...m, [subjectId]: "" }));
  };

  const removeChapter = (subjectId: string, chapter: string) => {
    setForm((f) => ({
      ...f,
      subjects: f.subjects.map((s) => (s.id === subjectId ? { ...s, chapters: s.chapters.filter((c) => c !== chapter) } : s)),
    }));
  };

  // Perubahan juga ditulis ke array `programs` di mock-data supaya halaman lain
  // (katalog, editor soal, dsb.) ikut membaca program yang di-set admin.
  const handleSave = () => {
    if (editing) {
      setRows((prev) => prev.map((r) => (r.id === editing.id ? { ...r, ...form } : r)));
      const idx = initialPrograms.findIndex((p) => p.id === editing.id);
      if (idx !== -1) initialPrograms[idx] = { ...initialPrograms[idx], ...form };
      toast.success(t.toastUpdated);
    } else {
      const created = { id: `prog-${Date.now()}`, ...form };
      setRows((prev) => [...prev, created]);
      initialPrograms.push(created);
      toast.success(t.toastCreated);
    }
    setModalOpen(false);
  };

  const handleDelete = (row: ProgramRow) => {
    const inUse = tryouts.some((tr) => tr.programId === row.id) || products.some((p) => p.programId === row.id);
    if (inUse) {
      toast.danger(t.toastCannotDeleteTitle, { description: t.toastCannotDeleteDescription });
      return;
    }
    setRows((prev) => prev.filter((r) => r.id !== row.id));
    const idx = initialPrograms.findIndex((p) => p.id === row.id);
    if (idx !== -1) initialPrograms.splice(idx, 1);
    toast.success(t.toastDeleted);
  };

  const columns: Column<ProgramRow>[] = [
    { key: "code", header: t.columns.code, render: (r) => <span className="font-mono text-xs font-medium text-accent">{r.code}</span> },
    { key: "name", header: t.columns.name, render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "description", header: t.columns.description, render: (r) => <span className="text-muted">{r.description}</span> },
    { key: "subjects", header: t.columns.subjectsHeader, render: (r) => <span className="text-muted">{t.columns.subjects(r.subjects.length)}</span> },
    {
      key: "tryouts",
      header: t.columns.tryouts,
      render: (r) => <span className="text-muted">{tryouts.filter((tr) => tr.programId === r.id).length}</span>,
    },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (r) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly onPress={() => openEdit(r)}>
            <Pencil className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" onPress={() => handleDelete(r)}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin"]} title={t.requireRoleTitle} deniedMessage={t.requireRoleDenied}>
    <AppShell
      title={t.title}
      description={t.description}
      actions={
        <Button variant="primary" size="sm" onPress={openCreate}>
          <Plus className="size-3.5" /> {t.addProgram}
        </Button>
      }
    >
      {rows.length === 0 ? (
        <EmptyState icon={School} title={t.emptyTitle} action={<Button variant="primary" size="sm" onPress={openCreate}>{t.addProgram}</Button>} />
      ) : (
        <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} onRowClick={openEdit} />
      )}

      <FormModal
        isOpen={modalOpen}
        onOpenChange={setModalOpen}
        title={editing ? t.editTitle : t.createTitle}
        footer={
          <>
            <Button variant="ghost" onPress={() => setModalOpen(false)}>
              {t.cancel}
            </Button>
            <Button variant="primary" onPress={handleSave} isDisabled={!form.code || !form.name}>
              {t.save}
            </Button>
          </>
        }
      >
        <TextInput label={t.codeLabel} placeholder={t.codePlaceholder} required value={form.code} onChange={(e) => setForm((f) => ({ ...f, code: e.target.value.toUpperCase() }))} />
        <TextInput label={t.nameLabel} placeholder={t.namePlaceholder} required value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
        <TextareaInput label={t.descriptionLabel} value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} rows={3} />

        <div>
          <p className="mb-1.5 text-[13px] font-medium text-foreground">{t.subjectsHeading}</p>
          <p className="mb-2 text-xs text-muted">{t.subjectsHelp}</p>
          <div className="flex gap-2">
            <TextInput
              className="flex-1"
              placeholder={t.subjectPlaceholder}
              value={subjectInput}
              onChange={(e) => setSubjectInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addSubject();
                }
              }}
            />
            <Button type="button" variant="secondary" onPress={addSubject} isDisabled={!subjectInput.trim()}>
              <Plus className="size-3.5" /> {t.addSubject}
            </Button>
          </div>
          {form.subjects.length > 0 ? (
            <div className="mt-2.5 space-y-2.5">
              {form.subjects.map((s) => (
                <div key={s.id} className="rounded-lg border border-border p-3">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium text-foreground">{s.name}</span>
                    <button
                      type="button"
                      onClick={() => removeSubject(s.id)}
                      className="rounded-full p-0.5 text-muted hover:bg-danger-soft hover:text-danger-soft-foreground"
                    >
                      <X className="size-3.5" />
                    </button>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {s.chapters.map((c) => (
                      <span
                        key={c}
                        className="inline-flex items-center gap-1 rounded-full bg-accent-soft py-1 pr-1.5 pl-2.5 text-xs font-medium text-accent-soft-foreground"
                      >
                        {c}
                        <button
                          type="button"
                          onClick={() => removeChapter(s.id, c)}
                          className="rounded-full p-0.5 text-accent-soft-foreground/70 hover:bg-accent/20 hover:text-accent-soft-foreground"
                        >
                          <X className="size-3" />
                        </button>
                      </span>
                    ))}
                  </div>
                  <div className="mt-2 flex gap-1.5">
                    <TextInput
                      className="flex-1"
                      placeholder={t.chapterPlaceholder}
                      value={chapterInputBySubject[s.id] ?? ""}
                      onChange={(e) => setChapterInputBySubject((m) => ({ ...m, [s.id]: e.target.value }))}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          addChapter(s.id);
                        }
                      }}
                    />
                    <Button type="button" variant="ghost" size="sm" onPress={() => addChapter(s.id)} isDisabled={!(chapterInputBySubject[s.id] ?? "").trim()}>
                      <Plus className="size-3.5" /> {t.addChapter}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="mt-2 text-xs text-muted">{t.noSubjects}</p>
          )}
        </div>
      </FormModal>
    </AppShell>
    </RequireRole>
  );
}
