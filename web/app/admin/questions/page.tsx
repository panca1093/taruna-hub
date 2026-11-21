"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, CloudOff, Eye, FileQuestion, Pencil, Plus, Trash2, Upload } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { RichDoc as RichDocView, RichDocSummary } from "@/components/rich-doc";
import { DifficultyBadge, ProgramBadge, QuestionTypeBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput } from "@/components/ui/form";
import { QuestionEditor, type QuestionDraft, draftOf, emptyDraft } from "@/components/admin/question-editor";
import { ImportQuestionsModal } from "@/components/admin/import-questions-modal";
import { api, ApiError, errorText, type Program, type Question } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { isDocEmpty, type RichDoc } from "@/lib/rich-doc";
import { useAdminQuestionsPageT } from "@/lib/i18n/dict/admin-questions-page";

const PAGE_SIZE = 50;

interface PreviewView {
  type: Question["type"];
  content: RichDoc;
  options: { id: string; label: RichDoc; points?: number }[];
  correct: string[];
  optionWeighted: boolean;
  explanation: RichDoc;
}

const viewOfQuestion = (question: Question): PreviewView => ({
  type: question.type,
  content: question.content,
  options: question.options,
  correct: question.correct,
  optionWeighted: question.option_weighted,
  explanation: question.explanation,
});

// Mirrors what bodyOf sends: empty options are dropped, except the fixed true/false pair.
const viewOfDraft = (draft: QuestionDraft): PreviewView => ({
  type: draft.type,
  content: draft.content,
  options: draft.options
    .filter((option) => draft.type === "benar_salah" || !isDocEmpty(option.label))
    .map((option, index) => ({ id: option.id ?? `option-${index}`, label: option.label, points: option.points })),
  correct: draft.type === "isian" ? draft.correct.map((answer) => answer.trim()).filter((answer) => answer !== "") : draft.correct,
  optionWeighted: draft.type === "pg" && draft.optionWeighted,
  explanation: draft.explanation,
});

function QuestionPreview({
  view,
  header,
  correctAnswerLabel,
  explanationLabel,
}: {
  view: PreviewView;
  header: ReactNode;
  correctAnswerLabel: string;
  explanationLabel: string;
}) {
  return (
    <>
      {header}
      <RichDocView doc={view.content} className="leading-relaxed text-foreground" />
      {view.type === "isian" ? (
        <div className="rounded-control border border-success/30 bg-success-soft px-3.5 py-3 text-small">
          <p className="mb-1 text-label font-medium tracking-wide text-success-soft-foreground uppercase">{correctAnswerLabel}</p>
          {view.correct.map((answer) => (
            <p key={answer} className="font-medium text-success-soft-foreground">{answer}</p>
          ))}
        </div>
      ) : view.optionWeighted ? (
        <div className="space-y-2">
          {view.options.map((option) => (
            <div key={option.id} className="flex items-center gap-2.5 rounded-control border border-border px-3.5 py-2.5 text-small">
              <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-accent-soft text-caption font-semibold text-accent-soft-foreground">
                {option.points ?? 1}
              </span>
              <RichDocView doc={option.label} className="min-w-0 flex-1 text-foreground" />
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {view.options.map((option) => {
            const correct = view.correct.includes(option.id);
            return (
              <div
                key={option.id}
                className={cn(
                  "flex items-center gap-2.5 rounded-control border px-3.5 py-2.5 text-small",
                  correct ? "border-success/40 bg-success-soft" : "border-border",
                )}
              >
                {correct ? <CheckCircle2 className="size-4 shrink-0 text-success" /> : <span className="size-4 shrink-0" />}
                <RichDocView doc={option.label} className={cn("min-w-0 flex-1", correct ? "font-medium text-success-soft-foreground" : "text-foreground")} />
              </div>
            );
          })}
        </div>
      )}
      {!isDocEmpty(view.explanation) && (
        <div className="rounded-control border border-dashed border-border bg-surface-secondary px-3.5 py-3 text-small">
          <p className="mb-1 text-label font-medium tracking-wide text-muted uppercase">{explanationLabel}</p>
          <RichDocView doc={view.explanation} />
        </div>
      )}
    </>
  );
}

export default function QuestionsPage() {
  const t = useAdminQuestionsPageT();

  const [programs, setPrograms] = useState<Program[] | null>(null);
  const [rows, setRows] = useState<Question[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const [programFilter, setProgramFilter] = useState("all");
  const [typeFilter, setTypeFilter] = useState<"all" | Question["type"]>("all");
  const [difficultyFilter, setDifficultyFilter] = useState<"all" | Question["difficulty"]>("all");

  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Question | null>(null);
  const [draftInitial, setDraftInitial] = useState<QuestionDraft | null>(null);
  const [editorSeq, setEditorSeq] = useState(0);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Question | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [preview, setPreview] = useState<Question | null>(null);
  const [draftPreview, setDraftPreview] = useState<QuestionDraft | null>(null);
  const latestDraft = useRef<QuestionDraft | null>(null);
  const trackDraft = useCallback((draft: QuestionDraft) => {
    latestDraft.current = draft;
  }, []);

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

  const programOptions = [{ id: "all", label: t.filterAllProgram }, ...(programs ?? []).map((program) => ({ id: program.id, label: program.name }))];

  const filters = {
    ...(programFilter === "all" ? {} : { program_id: programFilter }),
    ...(typeFilter === "all" ? {} : { type: typeFilter }),
    ...(difficultyFilter === "all" ? {} : { difficulty: difficultyFilter }),
  };

  useEffect(() => {
    let cancelled = false;
    api.programs
      .list()
      .then(({ programs }) => {
        if (!cancelled) setPrograms(programs);
      })
      .catch(() => {
        if (!cancelled) setPrograms([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Same pattern as app/admin/programs: the promise settles into state, never during the effect body.
  useEffect(() => {
    let cancelled = false;
    api.questions
      .list({ ...filters, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.questions);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [programFilter, typeFilter, difficultyFilter, reloadKey]);

  // A new filter starts from the loading state; doing it in the handler keeps the effect free of
  // synchronous state updates.
  const refilter = (apply: () => void) => {
    setRows(null);
    setLoadFailed(false);
    apply();
  };

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const page = await api.questions.list({ ...filters, limit: PAGE_SIZE, offset: rows?.length ?? 0 });
      setRows((prev) => [...(prev ?? []), ...page.questions]);
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const programOf = (id: string) => programs?.find((program) => program.id === id);
  const subjectName = (question: Question) =>
    programOf(question.program_id)?.subjects.find((subject) => subject.id === question.subject_id)?.name ?? "";
  const chapterName = (question: Question) => {
    if (!question.chapter_id) return "";
    const subjects = programOf(question.program_id)?.subjects ?? [];
    for (const subject of subjects) {
      const chapter = subject.chapters.find((candidate) => candidate.id === question.chapter_id);
      if (chapter) return chapter.name;
    }
    return "";
  };

  const openCreate = () => {
    const preferred = programFilter === "all" ? programs?.[0] : programOf(programFilter);
    setEditing(null);
    setFormErrors({});
    setDraftInitial(emptyDraft(preferred?.id ?? "", preferred?.subjects[0]?.id ?? ""));
    setEditorSeq((seq) => seq + 1);
    setEditorOpen(true);
  };

  const openEdit = (question: Question) => {
    setEditing(question);
    setFormErrors({});
    setDraftInitial(draftOf(question));
    setEditorSeq((seq) => seq + 1);
    setEditorOpen(true);
  };

  // bodyOf drops empty options, so the server's options[i] counts the kept ones; the form keys errors by draft position.
  const draftFieldErrors = (draft: QuestionDraft, fields: Record<string, string>) => {
    if (draft.type === "benar_salah") return fields;
    const kept = draft.options.flatMap((option, index) => (isDocEmpty(option.label) ? [] : [index]));
    return Object.fromEntries(
      Object.entries(fields).map(([key, message]) => [key.replace(/^options\[(\d+)\]/, (_, i: string) => `options[${kept[Number(i)] ?? Number(i)}]`), message]),
    );
  };

  // The body only names ids the server gave back; a new option's placeholder is resolved server-side.
  const bodyOf = (draft: QuestionDraft) => {
    const options = draft.type === "benar_salah" ? draft.options : draft.options.filter((option) => !isDocEmpty(option.label));
    const keptIds = new Set(options.map((option) => option.id));
    return {
      program_id: draft.programId,
      subject_id: draft.subjectId,
      chapter_id: draft.chapterId === "" ? null : draft.chapterId,
      type: draft.type,
      content: draft.content,
      difficulty: draft.difficulty,
      explanation: draft.explanation,
      option_weighted: draft.type === "pg" ? draft.optionWeighted : false,
      case_sensitive: draft.type === "isian" ? draft.caseSensitive : false,
      options: draft.type === "isian" ? [] : options.map((option) => ({
        ...(option.id ? { id: option.id } : {}),
        label: option.label,
        ...(option.points !== undefined ? { points: option.points } : {}),
      })),
      correct: draft.type === "isian" ? draft.correct.map((answer) => answer.trim()).filter((answer) => answer !== "") : draft.correct.filter((id) => id !== "" && keptIds.has(id)),
    };
  };

  const handleSave = async (draft: QuestionDraft) => {
    setSaving(true);
    setFormErrors({});
    try {
      if (editing) {
        await api.questions.update(editing.id, { ...bodyOf(draft), base_version: editing.version });
        toast.success(t.toastUpdated);
      } else {
        await api.questions.create(bodyOf(draft));
        toast.success(t.toastCreated);
      }
      setEditorOpen(false);
      setReloadKey((key) => key + 1);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.danger(t.toastConflictTitle, { description: err.message });
        // Keep the editor open with the draft, but refresh the bank behind it so the next edit starts
        // from the version someone else saved.
        setReloadKey((key) => key + 1);
      } else {
        if (err instanceof ApiError && err.fields) setFormErrors(draftFieldErrors(draft, err.fields));
        toast.danger(t.toastSaveFailed, { description: errorText(err, t.loadErrorDescription) });
      }
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.questions.remove(deleteTarget.id);
      setRows((prev) => prev?.filter((question) => question.id !== deleteTarget.id) ?? []);
      setTotal((count) => Math.max(0, count - 1));
      toast.success(t.toastDeleted);
      setDeleteTarget(null);
    } catch (err) {
      toast.danger(t.toastDeleteFailedTitle, { description: errorText(err, t.toastDeleteFailedDescription) });
    } finally {
      setDeleting(false);
    }
  };

  const columns: Column<Question>[] = [
    { key: "id", header: t.columns.id, render: (question) => <span className="font-mono text-caption text-muted">{question.number}</span> },
    {
      key: "content",
      header: t.columns.content,
      className: "max-w-md",
      render: (question) => (
        <div className="line-clamp-2 text-small">
          <RichDocSummary doc={question.content} />
        </div>
      ),
    },
    {
      key: "program",
      header: t.columns.program,
      render: (question) => {
        const program = programOf(question.program_id);
        return program ? <ProgramBadge code={program.code} /> : null;
      },
    },
    {
      key: "subject",
      header: t.columns.subject,
      render: (question) => (
        <div>
          <span className="text-muted">{subjectName(question)}</span>
          {chapterName(question) && <p className="text-micro text-muted/70">{chapterName(question)}</p>}
        </div>
      ),
    },
    { key: "type", header: t.columns.type, render: (question) => <QuestionTypeBadge type={question.type} /> },
    { key: "difficulty", header: t.columns.difficulty, render: (question) => <DifficultyBadge difficulty={question.difficulty} /> },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (question) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly onPress={() => openEdit(question)}>
            <Pencil className="size-4" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" onPress={() => setDeleteTarget(question)}>
            <Trash2 className="size-4" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin", "operator"]} title={t.requireRoleTitle} deniedMessage={t.requireRoleDenied}>
      <AppShell
        title={t.title}
        description={t.description(total)}
        actions={
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onPress={() => setImportOpen(true)}>
              <Upload className="size-3.5" /> <span className="sr-only sm:not-sr-only">{t.bulkImport}</span>
            </Button>
            <Button variant="primary" size="sm" onPress={openCreate}>
              <Plus className="size-4" /> <span className="sr-only sm:not-sr-only">{t.addQuestion}</span>
            </Button>
          </div>
        }
      >
        <div className="mb-4 flex flex-wrap gap-3">
          <SelectInput className="w-44" options={programOptions} selectedKey={programFilter} onSelectionChange={(key) => refilter(() => setProgramFilter(key ?? "all"))} />
          <SelectInput className="w-44" options={typeOptions} selectedKey={typeFilter} onSelectionChange={(key) => refilter(() => setTypeFilter((key ?? "all") as "all" | Question["type"]))} />
          <SelectInput className="w-44" options={difficultyOptions} selectedKey={difficultyFilter} onSelectionChange={(key) => refilter(() => setDifficultyFilter((key ?? "all") as "all" | Question["difficulty"]))} />
        </div>

        {loadFailed ? (
          <EmptyState
            icon={CloudOff}
            title={t.loadErrorTitle}
            description={t.loadErrorDescription}
            action={<Button variant="primary" size="sm" onPress={retry}>{t.retry}</Button>}
          />
        ) : rows === null ? (
          <p className="py-14 text-center text-small text-muted">{t.loading}</p>
        ) : rows.length === 0 ? (
          <EmptyState icon={FileQuestion} title={t.emptyTitle} description={t.emptyDescription} action={<Button variant="primary" size="sm" onPress={openCreate}>{t.addQuestion}</Button>} />
        ) : (
          <>
            <DataTable columns={columns} rows={rows} rowKey={(question) => question.id} footer={<span>{t.footerCount(rows.length, total)}</span>} onRowClick={setPreview} />
            {rows.length < total && (
              <div className="mt-4 flex justify-center">
                <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                  {loadingMore ? t.loadingMore : t.loadMore}
                </Button>
              </div>
            )}
          </>
        )}

        <FormModal
          isOpen={editorOpen}
          onOpenChange={setEditorOpen}
          title={editing ? t.editTitle : t.createTitle}
          size="xl"
          footer={
            <div className="flex w-full items-center gap-2">
              <Button variant="secondary" className="mr-auto" aria-label={t.previewDraft} onPress={() => setDraftPreview(latestDraft.current)}>
                <Eye className="size-4" /> <span className="hidden sm:inline">{t.previewDraft}</span>
              </Button>
              <Button variant="ghost" onPress={() => setEditorOpen(false)}>
                {t.cancel}
              </Button>
              <Button variant="primary" type="submit" form="question-editor-form" isDisabled={saving}>
                {saving ? t.saving : t.saveQuestion}
              </Button>
            </div>
          }
        >
          {draftInitial && (
            <QuestionEditor
              key={`${editing?.id ?? "new"}-${editorSeq}`}
              initial={draftInitial}
              programs={programs ?? []}
              errors={formErrors}
              saving={saving}
              onSubmit={(draft) => void handleSave(draft)}
              onDraftChange={trackDraft}
            />
          )}
        </FormModal>

        <FormModal
          isOpen={!!draftPreview}
          onOpenChange={(open) => !open && setDraftPreview(null)}
          title={t.draftPreviewTitle}
          size="lg"
          footer={
            <Button variant="ghost" onPress={() => setDraftPreview(null)}>
              {t.close}
            </Button>
          }
        >
          {draftPreview && (
            <QuestionPreview
              view={viewOfDraft(draftPreview)}
              correctAnswerLabel={t.correctAnswer}
              explanationLabel={t.explanation}
              header={
                <div className="flex flex-wrap items-center gap-1.5">
                  {programOf(draftPreview.programId) && <ProgramBadge code={programOf(draftPreview.programId)!.code} />}
                  <QuestionTypeBadge type={draftPreview.type} />
                  <DifficultyBadge difficulty={draftPreview.difficulty} />
                  {draftPreview.type === "pg" && draftPreview.optionWeighted && (
                    <span className="text-caption text-muted">{t.optionWeighted}</span>
                  )}
                </div>
              }
            />
          )}
        </FormModal>

        <ImportQuestionsModal
          isOpen={importOpen}
          onOpenChange={setImportOpen}
          onImported={() => setReloadKey((key) => key + 1)}
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
                <Pencil className="size-4" /> {t.editQuestion}
              </Button>
            </>
          }
        >
          {preview && (
            <QuestionPreview
              view={viewOfQuestion(preview)}
              correctAnswerLabel={t.correctAnswer}
              explanationLabel={t.explanation}
              header={
                <div className="flex flex-wrap items-center gap-1.5">
                  {programOf(preview.program_id) && <ProgramBadge code={programOf(preview.program_id)!.code} />}
                  <QuestionTypeBadge type={preview.type} />
                  <DifficultyBadge difficulty={preview.difficulty} />
                  <span className="text-caption text-muted">
                    {subjectName(preview)}
                    {chapterName(preview) ? ` / ${chapterName(preview)}` : ""} · ID {preview.number} ·{" "}
                    {t.versionLabel(preview.version)}
                    {preview.option_weighted ? ` · ${t.optionWeighted}` : ""}
                  </span>
                </div>
              }
            />
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
              <Button variant="danger" onPress={confirmDelete} isDisabled={deleting}>
                {t.confirmDelete}
              </Button>
            </>
          }
        >
          {deleteTarget && (
            <p className="text-small text-foreground">
              <RichDocSummary doc={deleteTarget.content} />
            </p>
          )}
        </FormModal>
      </AppShell>
    </RequireRole>
  );
}
