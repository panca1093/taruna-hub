"use client";

import { useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CloudOff, Pencil, Plus, School, Trash2, X } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput, TextareaInput } from "@/components/ui/form";
import { api, ApiError, errorText, type Program } from "@/lib/api/client";
import { useAdminProgramsT } from "@/lib/i18n/dict/admin-programs";

// The form keeps the ids the server gave it, so a save updates those rows instead of replacing them.
interface ChapterDraft {
  id?: string;
  name: string;
}

interface SubjectDraft {
  id?: string;
  name: string;
  chapters: ChapterDraft[];
}

interface ProgramRow {
  id: string;
  code: string;
  name: string;
  description: string;
  subjects: SubjectDraft[];
}

const toRow = (program: Program): ProgramRow => ({
  id: program.id,
  code: program.code,
  name: program.name,
  description: program.description,
  subjects: program.subjects.map((subject) => ({
    id: subject.id,
    name: subject.name,
    chapters: subject.chapters.map((chapter) => ({ id: chapter.id, name: chapter.name })),
  })),
});

const emptyForm = () => ({ code: "", name: "", description: "", subjects: [] as SubjectDraft[] });

export default function ProgramsPage() {
  const t = useAdminProgramsT();
  const [rows, setRows] = useState<ProgramRow[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [editing, setEditing] = useState<ProgramRow | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [subjectInput, setSubjectInput] = useState("");
  const [chapterInputBySubject, setChapterInputBySubject] = useState<Record<number, string>>({});
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  // Same pattern as app/dev/api-check: the promise settles into state, never during the effect body.
  useEffect(() => {
    let cancelled = false;
    api.programs
      .list()
      .then(({ programs }) => {
        if (cancelled) return;
        setRows(programs.map(toRow));
        setLoadFailed(false);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const openCreate = () => {
    setEditing(null);
    setForm(emptyForm());
    setFormErrors({});
    setSubjectInput("");
    setChapterInputBySubject({});
    setModalOpen(true);
  };

  const openEdit = (row: ProgramRow) => {
    setEditing(row);
    setForm({
      code: row.code,
      name: row.name,
      description: row.description,
      subjects: row.subjects.map((subject) => ({
        ...subject,
        chapters: subject.chapters.map((chapter) => ({ ...chapter })),
      })),
    });
    setFormErrors({});
    setSubjectInput("");
    setChapterInputBySubject({});
    setModalOpen(true);
  };

  const addSubject = () => {
    const value = subjectInput.trim();
    if (!value || form.subjects.some((s) => s.name.toLowerCase() === value.toLowerCase())) {
      setSubjectInput("");
      return;
    }
    setForm((f) => ({ ...f, subjects: [...f.subjects, { name: value, chapters: [] }] }));
    setSubjectInput("");
  };

  const removeSubject = (index: number) => {
    setForm((f) => ({ ...f, subjects: f.subjects.filter((_, i) => i !== index) }));
  };

  const addChapter = (subjectIndex: number) => {
    const value = (chapterInputBySubject[subjectIndex] ?? "").trim();
    if (!value) return;
    setForm((f) => ({
      ...f,
      subjects: f.subjects.map((s, i) =>
        i === subjectIndex && !s.chapters.some((c) => c.name.toLowerCase() === value.toLowerCase())
          ? { ...s, chapters: [...s.chapters, { name: value }] }
          : s,
      ),
    }));
    setChapterInputBySubject((m) => ({ ...m, [subjectIndex]: "" }));
  };

  const removeChapter = (subjectIndex: number, chapterIndex: number) => {
    setForm((f) => ({
      ...f,
      subjects: f.subjects.map((s, i) =>
        i === subjectIndex ? { ...s, chapters: s.chapters.filter((_, j) => j !== chapterIndex) } : s,
      ),
    }));
  };

  // The PUT body carries the ids of what is kept; a create body must not mention ids at all.
  const replaceBody = () => ({
    code: form.code,
    name: form.name,
    description: form.description,
    subjects: form.subjects.map((subject) => ({
      ...(subject.id ? { id: subject.id } : {}),
      name: subject.name,
      chapters: subject.chapters.map((chapter) => ({
        ...(chapter.id ? { id: chapter.id } : {}),
        name: chapter.name,
      })),
    })),
  });

  const createBody = () => ({
    code: form.code,
    name: form.name,
    description: form.description,
    subjects: form.subjects.map((subject) => ({
      name: subject.name,
      chapters: subject.chapters.map((chapter) => ({ name: chapter.name })),
    })),
  });

  const handleSave = async () => {
    setSaving(true);
    setFormErrors({});
    try {
      if (editing) {
        const saved = await api.programs.update(editing.id, replaceBody());
        setRows((prev) => prev?.map((r) => (r.id === saved.id ? toRow(saved) : r)) ?? [toRow(saved)]);
        toast.success(t.toastUpdated);
      } else {
        const created = await api.programs.create(createBody());
        setRows((prev) => [...(prev ?? []), toRow(created)]);
        toast.success(t.toastCreated);
      }
      setModalOpen(false);
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFormErrors(err.fields);
      toast.danger(t.toastSaveFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (row: ProgramRow) => {
    try {
      await api.programs.remove(row.id);
      setRows((prev) => prev?.filter((r) => r.id !== row.id) ?? []);
      toast.success(t.toastDeleted);
    } catch (err) {
      toast.danger(t.toastCannotDeleteTitle, { description: errorText(err, t.toastCannotDeleteDescription) });
    }
  };

  const columns: Column<ProgramRow>[] = [
    { key: "code", header: t.columns.code, render: (r) => <span className="font-mono text-xs font-medium text-accent">{r.code}</span> },
    { key: "name", header: t.columns.name, render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "description", header: t.columns.description, render: (r) => <span className="text-muted">{r.description}</span> },
    { key: "subjects", header: t.columns.subjectsHeader, render: (r) => <span className="text-muted">{t.columns.subjects(r.subjects.length)}</span> },
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
      {loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={<Button variant="primary" size="sm" onPress={retry}>{t.retry}</Button>}
        />
      ) : rows === null ? (
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      ) : rows.length === 0 ? (
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
            <Button variant="primary" onPress={handleSave} isDisabled={!form.code || !form.name || saving}>
              {saving ? t.saving : t.save}
            </Button>
          </>
        }
      >
        <TextInput
          label={t.codeLabel}
          placeholder={t.codePlaceholder}
          required
          error={formErrors.code}
          value={form.code}
          onChange={(e) => setForm((f) => ({ ...f, code: e.target.value.toUpperCase() }))}
        />
        <TextInput
          label={t.nameLabel}
          placeholder={t.namePlaceholder}
          required
          error={formErrors.name}
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
        />
        <TextareaInput
          label={t.descriptionLabel}
          error={formErrors.description}
          value={form.description}
          onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
          rows={3}
        />

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
              {form.subjects.map((s, subjectIndex) => (
                <div key={s.id ?? `new-subject-${subjectIndex}`} className="rounded-lg border border-border p-3">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium text-foreground">{s.name}</span>
                    <button
                      type="button"
                      onClick={() => removeSubject(subjectIndex)}
                      className="rounded-full p-0.5 text-muted hover:bg-danger-soft hover:text-danger-soft-foreground"
                    >
                      <X className="size-3.5" />
                    </button>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {s.chapters.map((c, chapterIndex) => (
                      <span
                        key={c.id ?? `${c.name}-${chapterIndex}`}
                        className="inline-flex items-center gap-1 rounded-full bg-accent-soft py-1 pr-1.5 pl-2.5 text-xs font-medium text-accent-soft-foreground"
                      >
                        {c.name}
                        <button
                          type="button"
                          onClick={() => removeChapter(subjectIndex, chapterIndex)}
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
                      value={chapterInputBySubject[subjectIndex] ?? ""}
                      onChange={(e) => setChapterInputBySubject((m) => ({ ...m, [subjectIndex]: e.target.value }))}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          addChapter(subjectIndex);
                        }
                      }}
                    />
                    <Button type="button" variant="ghost" size="sm" onPress={() => addChapter(subjectIndex)} isDisabled={!(chapterInputBySubject[subjectIndex] ?? "").trim()}>
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
