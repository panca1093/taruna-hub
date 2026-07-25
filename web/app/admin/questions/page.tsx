"use client";

import { useMemo, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, FileQuestion, Pencil, Plus, Trash2, Upload } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { LatexText } from "@/components/latex";
import { DifficultyBadge, ProgramBadge, QuestionTypeBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput } from "@/components/ui/form";
import { QuestionEditor, type QuestionDraft, emptyDraft } from "@/components/admin/question-editor";
import { ImportQuestionsModal } from "@/components/admin/import-questions-modal";
import { getProgram, programs, questions as initialQuestions } from "@/lib/mock-data";
import type { Question } from "@/lib/types";
import { cn } from "@/lib/cn";
import { useAdminQuestionsPageT } from "@/lib/i18n/dict/admin-questions-page";

export default function QuestionsPage() {
  const t = useAdminQuestionsPageT();

  const typeOptions = [
    { id: "all", label: t.filterAllType },
    { id: "pg", label: t.questionType.pg },
    { id: "pg_multi", label: t.questionType.pg_multi },
    { id: "isian", label: t.questionType.isian },
    { id: "benar_salah", label: t.questionType.benar_salah },
  ];

  const difficultyOptions = [
    { id: "all", label: t.filterAllDifficulty },
    { id: "easy", label: t.difficulty.easy },
    { id: "medium", label: t.difficulty.medium },
    { id: "hard", label: t.difficulty.hard },
  ];

  const programOptions = [{ id: "all", label: t.filterAllProgram }, ...programs.map((p) => ({ id: p.id, label: p.name }))];

  const [rows, setRows] = useState<Question[]>(initialQuestions);
  const [programFilter, setProgramFilter] = useState("all");
  const [typeFilter, setTypeFilter] = useState("all");
  const [difficultyFilter, setDifficultyFilter] = useState("all");
  const [editorOpen, setEditorOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [editing, setEditing] = useState<Question | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Question | null>(null);
  const [preview, setPreview] = useState<Question | null>(null);

  const filtered = useMemo(
    () =>
      rows.filter(
        (q) =>
          (programFilter === "all" || q.programId === programFilter) &&
          (typeFilter === "all" || q.type === typeFilter) &&
          (difficultyFilter === "all" || q.difficulty === difficultyFilter),
      ),
    [rows, programFilter, typeFilter, difficultyFilter],
  );

  const openCreate = () => {
    setEditing(null);
    setEditorOpen(true);
  };

  const openEdit = (q: Question) => {
    setEditing(q);
    setEditorOpen(true);
  };

  const handleSave = (draft: QuestionDraft) => {
    if (editing) {
      setRows((prev) => prev.map((q) => (q.id === editing.id ? { ...editing, ...draft } : q)));
      toast.success(t.toastUpdated);
    } else {
      const nextId = Math.max(0, ...rows.map((q) => Number(q.id.replace(/\D/g, "")) || 0)) + 1;
      setRows((prev) => [{ id: `q${nextId}`, ...draft }, ...prev]);
      toast.success(t.toastCreated);
    }
    setEditorOpen(false);
  };

  const columns: Column<Question>[] = [
    { key: "id", header: t.columns.id, render: (q) => <span className="font-mono text-xs text-muted">{q.id.replace(/\D/g, "")}</span> },
    {
      key: "content",
      header: t.columns.content,
      className: "max-w-md",
      render: (q) => (
        <div className="line-clamp-2 text-sm">
          <LatexText text={q.content} />
        </div>
      ),
    },
    { key: "program", header: t.columns.program, render: (q) => { const p = getProgram(q.programId); return p ? <ProgramBadge code={p.code} /> : null; } },
    {
      key: "subject",
      header: t.columns.subject,
      render: (q) => (
        <div>
          <span className="text-muted">{q.subject}</span>
          {q.chapter && <p className="text-[10px] text-muted/70">{q.chapter}</p>}
        </div>
      ),
    },
    { key: "type", header: t.columns.type, render: (q) => <QuestionTypeBadge type={q.type} /> },
    { key: "difficulty", header: t.columns.difficulty, render: (q) => <DifficultyBadge difficulty={q.difficulty} /> },
    { key: "weight", header: t.columns.weight, render: (q) => <span className="font-mono text-xs">{q.weight}</span> },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (q) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly onPress={() => openEdit(q)}>
            <Pencil className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" onPress={() => setDeleteTarget(q)}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <AppShell
      title={t.title}
      description={t.description(rows.length)}
      actions={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onPress={() => setImportOpen(true)}>
            <Upload className="size-3.5" /> {t.bulkImport}
          </Button>
          <Button variant="primary" size="sm" onPress={openCreate}>
            <Plus className="size-3.5" /> {t.addQuestion}
          </Button>
        </div>
      }
    >
      <div className="mb-4 flex flex-wrap gap-3">
        <SelectInput className="w-44" options={programOptions} selectedKey={programFilter} onSelectionChange={(k) => setProgramFilter(k ?? "all")} />
        <SelectInput className="w-44" options={typeOptions} selectedKey={typeFilter} onSelectionChange={(k) => setTypeFilter(k ?? "all")} />
        <SelectInput className="w-44" options={difficultyOptions} selectedKey={difficultyFilter} onSelectionChange={(k) => setDifficultyFilter(k ?? "all")} />
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={FileQuestion} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <DataTable columns={columns} rows={filtered} rowKey={(q) => q.id} onRowClick={setPreview} />
      )}

      <FormModal
        isOpen={editorOpen}
        onOpenChange={setEditorOpen}
        title={editing ? t.editTitle : t.createTitle}
        size="xl"
        footer={
          <>
            <Button variant="ghost" onPress={() => setEditorOpen(false)}>
              {t.cancel}
            </Button>
            <Button variant="primary" type="submit" form="question-editor-form">
              {t.saveQuestion}
            </Button>
          </>
        }
      >
        <QuestionEditor initial={editing ? { ...editing } : emptyDraft(programFilter !== "all" ? programFilter : programs[0].id)} onSubmit={handleSave} />
      </FormModal>

      <ImportQuestionsModal
        isOpen={importOpen}
        onOpenChange={setImportOpen}
        onImported={(count) => {
          toast.success(t.importedToast(count));
        }}
      />

      <FormModal
        isOpen={!!preview}
        onOpenChange={(open) => !open && setPreview(null)}
        title={t.previewTitle}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onPress={() => setPreview(null)}>
              {t.close}
            </Button>
            <Button
              variant="primary"
              onPress={() => {
                if (preview) openEdit(preview);
                setPreview(null);
              }}
            >
              <Pencil className="size-3.5" /> {t.editQuestion}
            </Button>
          </>
        }
      >
        {preview && (
          <>
            <div className="flex flex-wrap items-center gap-1.5">
              {getProgram(preview.programId) && <ProgramBadge code={getProgram(preview.programId)!.code} />}
              <QuestionTypeBadge type={preview.type} />
              <DifficultyBadge difficulty={preview.difficulty} />
              <span className="text-xs text-muted">
                {preview.subject}
                {preview.chapter ? ` / ${preview.chapter}` : ""} · ID {preview.id.replace(/\D/g, "")} ·{" "}
                {preview.optionWeighted ? t.optionWeighted : t.weightLabel(preview.weight)}
              </span>
            </div>
            <div className="text-[15px] leading-relaxed text-foreground">
              <LatexText text={preview.content} />
            </div>
            {preview.type === "isian" ? (
              <div className="rounded-lg border border-success/30 bg-success-soft px-3.5 py-3 text-sm">
                <p className="mb-1 text-[11px] font-medium tracking-wide text-success-soft-foreground uppercase">{t.correctAnswer}</p>
                <p className="font-medium text-success-soft-foreground">{preview.correctAnswer[0]}</p>
              </div>
            ) : preview.optionWeighted ? (
              <div className="space-y-2">
                {preview.options?.map((opt) => (
                  <div key={opt.id} className="flex items-center gap-2.5 rounded-lg border border-border px-3.5 py-2.5 text-sm">
                    <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-accent-soft text-xs font-semibold text-accent-soft-foreground">
                      {opt.points ?? 1}
                    </span>
                    <LatexText text={opt.label} className="text-foreground" />
                  </div>
                ))}
              </div>
            ) : (
              <div className="space-y-2">
                {preview.options?.map((opt) => {
                  const correct = preview.correctAnswer.includes(opt.id);
                  return (
                    <div
                      key={opt.id}
                      className={cn(
                        "flex items-center gap-2.5 rounded-lg border px-3.5 py-2.5 text-sm",
                        correct ? "border-success/40 bg-success-soft" : "border-border",
                      )}
                    >
                      {correct ? <CheckCircle2 className="size-4 shrink-0 text-success" /> : <span className="size-4 shrink-0" />}
                      <LatexText text={opt.label} className={correct ? "font-medium text-success-soft-foreground" : "text-foreground"} />
                    </div>
                  );
                })}
              </div>
            )}
            <div className="rounded-lg border border-dashed border-border bg-surface-secondary px-3.5 py-3 text-sm">
              <p className="mb-1 text-[11px] font-medium tracking-wide text-muted uppercase">{t.explanation}</p>
              <LatexText text={preview.explanation} />
            </div>
          </>
        )}
      </FormModal>

      <FormModal
        isOpen={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t.deleteTitle}
        description={t.deleteDescription}
        footer={
          <>
            <Button variant="ghost" onPress={() => setDeleteTarget(null)}>
              {t.cancel}
            </Button>
            <Button
              variant="danger"
              onPress={() => {
                setRows((prev) => prev.filter((q) => q.id !== deleteTarget?.id));
                toast.success(t.toastDeleted);
                setDeleteTarget(null);
              }}
            >
              {t.confirmDelete}
            </Button>
          </>
        }
      >
        {deleteTarget && (
          <p className="text-sm text-foreground">
            <LatexText text={deleteTarget.content} />
          </p>
        )}
      </FormModal>
    </AppShell>
  );
}
