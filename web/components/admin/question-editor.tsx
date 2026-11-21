"use client";

import { useEffect, useMemo, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { RichTextField, RichTextGroup, RichTextToolbar } from "@/components/admin/rich-text-editor";
import { RichDoc as RichDocView } from "@/components/rich-doc";
import { SelectInput, SwitchInput, TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import type { Program, Question } from "@/lib/api/client";
import { docFromText, emptyDoc, type RichDoc } from "@/lib/rich-doc";
import { useAdminQuestionEditorT } from "@/lib/i18n/dict/admin-question-editor";

export type QuestionType = Question["type"];
export type QuestionDifficulty = Question["difficulty"];

// A new option carries a placeholder id; the server accepts it in `correct` and answers with real ids.
export interface DraftOption {
  id?: string;
  label: RichDoc;
  points?: number;
}

export interface QuestionDraft {
  programId: string;
  subjectId: string;
  chapterId: string;
  type: QuestionType;
  content: RichDoc;
  difficulty: QuestionDifficulty;
  explanation: RichDoc;
  optionWeighted: boolean;
  caseSensitive: boolean;
  options: DraftOption[];
  correct: string[];
}

const placeholderId = () => "new-" + Math.random().toString(36).slice(2, 10);

const blankOptions = (): DraftOption[] => [1, 2, 3, 4].map(() => ({ id: placeholderId(), label: emptyDoc() }));

export function emptyDraft(programId: string, subjectId: string): QuestionDraft {
  return {
    programId,
    subjectId,
    chapterId: "",
    type: "pg",
    content: emptyDoc(),
    difficulty: "easy",
    explanation: emptyDoc(),
    optionWeighted: false,
    caseSensitive: false,
    options: blankOptions(),
    correct: [],
  };
}

/** draftOf turns a question the API returned into the editable form, keeping every server id. */
export function draftOf(question: Question): QuestionDraft {
  return {
    programId: question.program_id,
    subjectId: question.subject_id,
    chapterId: question.chapter_id ?? "",
    type: question.type,
    content: question.content,
    difficulty: question.difficulty,
    explanation: question.explanation,
    optionWeighted: question.option_weighted,
    caseSensitive: question.case_sensitive,
    options: question.options.map((option) => ({ id: option.id, label: option.label, points: option.points })),
    correct: [...question.correct],
  };
}

const trueFalseOptions = (): DraftOption[] => [
  { id: "benar", label: docFromText("Benar") },
  { id: "salah", label: docFromText("Salah") },
];

const letterOf = (index: number) => String.fromCharCode(65 + index);

export function QuestionEditor({
  initial,
  programs,
  errors = {},
  saving = false,
  onSubmit,
  onDraftChange,
}: {
  initial: QuestionDraft;
  programs: Program[];
  errors?: Record<string, string>;
  saving?: boolean;
  onSubmit: (draft: QuestionDraft) => void;
  onDraftChange?: (draft: QuestionDraft) => void;
}) {
  const t = useAdminQuestionEditorT();
  const [draft, setDraft] = useState<QuestionDraft>(initial);

  useEffect(() => {
    onDraftChange?.(draft);
  }, [draft, onDraftChange]);

  const typeOptions = [
    { id: "pg", label: t.typeOptions.pg },
    { id: "pg_multi", label: t.typeOptions.pg_multi },
    { id: "isian", label: t.typeOptions.isian },
    { id: "benar_salah", label: t.typeOptions.benar_salah },
  ];

  const difficultyOptions = [
    { id: "easy", label: t.difficultyOptions.easy },
    { id: "medium", label: t.difficultyOptions.medium },
    { id: "hard", label: t.difficultyOptions.hard },
  ];

  const programOptions = programs.map((program) => ({ id: program.id, label: program.name }));
  const program = programs.find((candidate) => candidate.id === draft.programId);
  const subjectOptions = useMemo(
    () => (program?.subjects ?? []).map((subject) => ({ id: subject.id, label: subject.name })),
    [program],
  );
  const chapterOptions = useMemo(
    () => (program?.subjects.find((subject) => subject.id === draft.subjectId)?.chapters ?? []).map((chapter) => ({ id: chapter.id, label: chapter.name })),
    [program, draft.subjectId],
  );

  const setType = (type: QuestionType) => {
    if (type === "benar_salah") {
      setDraft((d) => ({ ...d, type, options: trueFalseOptions(), correct: [], optionWeighted: false }));
    } else if (type === "isian") {
      setDraft((d) => ({ ...d, type, options: [], correct: [], optionWeighted: false }));
    } else {
      setDraft((d) => ({
        ...d,
        type,
        options: d.options.length >= 2 ? d.options : blankOptions(),
        correct: [],
        optionWeighted: type === "pg" ? d.optionWeighted : false,
      }));
    }
  };

  const updateOption = (index: number, patch: Partial<DraftOption>) => {
    setDraft((d) => ({ ...d, options: d.options.map((option, i) => (i === index ? { ...option, ...patch } : option)) }));
  };

  const addOption = () => {
    setDraft((d) => ({ ...d, options: [...d.options, { id: placeholderId(), label: emptyDoc() }] }));
  };

  const removeOption = (index: number) => {
    setDraft((d) => {
      const removed = d.options[index];
      return {
        ...d,
        options: d.options.filter((_, i) => i !== index),
        correct: removed ? d.correct.filter((id) => id !== removed.id) : d.correct,
      };
    });
  };

  const toggleCorrect = (option: DraftOption) => {
    const key = option.id ?? "";
    setDraft((d) => {
      if (d.type === "pg_multi") {
        return {
          ...d,
          correct: d.correct.includes(key) ? d.correct.filter((id) => id !== key) : [...d.correct, key],
        };
      }
      return { ...d, correct: [key] };
    });
  };

  const answers = draft.correct.join("\n");

  return (
    <form
      id="question-editor-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (!saving) onSubmit(draft);
      }}
      className="w-full"
    >
      <RichTextGroup>
        <div className="grid gap-6 lg:grid-cols-question-form">
          <div className="flex min-w-0 flex-col gap-6">
            <RichTextToolbar />

            <section className="flex flex-col gap-2">
              <p className="text-small font-semibold text-foreground">
                {t.contentLabel} <span className="text-danger">*</span>
              </p>
              <RichTextField
                label={t.contentLabel}
                size="content"
                placeholder={t.contentPlaceholder}
                value={draft.content}
                onChange={(content) => setDraft((d) => ({ ...d, content }))}
                invalid={!!errors.content}
              />
              {errors.content && <p className="text-caption text-danger">{errors.content}</p>}
            </section>

            {draft.type === "isian" ? (
              <section className="flex flex-col gap-3">
                <TextareaInput
                  label={t.correctAnswerLabel}
                  description={t.correctAnswerDescription}
                  required
                  rows={3}
                  value={answers}
                  onChange={(e) => setDraft((d) => ({ ...d, correct: e.target.value.split("\n") }))}
                  error={errors.correct}
                />
                <SwitchInput
                  label={t.caseSensitiveLabel}
                  description={t.caseSensitiveDescription}
                  isSelected={draft.caseSensitive}
                  onChange={(checked) => setDraft((d) => ({ ...d, caseSensitive: checked }))}
                />
                {errors.options && <p className="text-caption text-danger">{errors.options}</p>}
                {errors.case_sensitive && <p className="text-caption text-danger">{errors.case_sensitive}</p>}
              </section>
            ) : (
              <section className="flex flex-col gap-2">
                <div className="flex flex-wrap items-baseline justify-between gap-x-2.5">
                  <p className="text-small font-semibold text-foreground">{t.optionsLabel}</p>
                  <span className="text-label text-muted">{draft.optionWeighted ? t.optionsHintWeighted : t.optionsHintMark}</span>
                </div>
                <div className="flex flex-col gap-2">
                  {draft.options.map((option, index) => {
                    const letter = letterOf(index);
                    const checked = option.id !== undefined && draft.correct.includes(option.id);
                    const readOnly = draft.type === "benar_salah";
                    const canRemove = !readOnly && draft.options.length > 2;
                    return (
                      <div key={option.id ?? `option-${index}`} className="flex flex-col gap-1">
                        <div className="flex items-start gap-2">
                          {draft.optionWeighted ? (
                            <input
                              type="number"
                              min={0}
                              max={100}
                              step={0.01}
                              aria-label={t.optionPoints(letter)}
                              value={option.points ?? ""}
                              placeholder="1"
                              onChange={(e) => updateOption(index, { points: e.target.value === "" ? undefined : Number(e.target.value) })}
                              className="mt-px h-10 w-15 shrink-0 rounded-control border border-border bg-field px-2 text-center text-caption font-semibold text-field-foreground outline-none focus:border-accent"
                            />
                          ) : (
                            <button
                              type="button"
                              onClick={() => toggleCorrect(option)}
                              aria-pressed={checked}
                              aria-label={t.markCorrect(letter)}
                              className={cn(
                                "mt-0.75 flex size-9 shrink-0 items-center justify-center border-2 text-caption font-semibold transition-colors",
                                draft.type === "pg_multi" ? "rounded-control" : "rounded-full",
                                checked
                                  ? "border-success bg-success text-success-foreground"
                                  : "border-border bg-surface text-muted hover:border-success hover:text-success",
                              )}
                            >
                              {letter}
                            </button>
                          )}
                          {readOnly ? (
                            <div className="min-w-0 flex-1 px-3 py-2 text-small text-foreground">
                              <RichDocView doc={option.label} />
                            </div>
                          ) : (
                            <RichTextField
                              className="min-w-0 flex-1"
                              label={t.optionField(letter)}
                              placeholder={t.optionPlaceholder}
                              value={option.label}
                              correct={checked && !draft.optionWeighted}
                              onChange={(label) => updateOption(index, { label })}
                              invalid={!!errors[`options[${index}].label`]}
                            />
                          )}
                          {!readOnly && (
                            <button
                              type="button"
                              aria-label={t.removeOption(letter)}
                              onClick={() => removeOption(index)}
                              disabled={!canRemove}
                              className={cn(
                                "mt-0.75 flex size-9 shrink-0 items-center justify-center rounded-chip text-muted transition-colors hover:bg-surface-tertiary hover:text-danger",
                                !canRemove && "invisible",
                              )}
                            >
                              <Trash2 className="size-4" />
                            </button>
                          )}
                        </div>
                        {errors[`options[${index}].label`] && (
                          <p className="pl-11 text-caption text-danger">{errors[`options[${index}].label`]}</p>
                        )}
                        {errors[`options[${index}].points`] && (
                          <p className="pl-11 text-caption text-danger">{errors[`options[${index}].points`]}</p>
                        )}
                      </div>
                    );
                  })}
                </div>
                {draft.type !== "benar_salah" && draft.options.length < 8 && (
                  <button
                    type="button"
                    onClick={addOption}
                    className="ml-11 inline-flex h-9 items-center gap-2 self-start rounded-control border border-dashed border-border px-3 text-caption font-medium text-accent transition-colors hover:bg-surface-secondary"
                  >
                    <Plus className="size-4" /> {t.addOption}
                  </button>
                )}
                {errors.correct && <p className="text-caption text-danger">{errors.correct}</p>}
                {errors.options && <p className="text-caption text-danger">{errors.options}</p>}
                {errors.option_weighted && <p className="text-caption text-danger">{errors.option_weighted}</p>}
              </section>
            )}

            <section className="flex flex-col gap-2">
              <div className="flex flex-wrap items-baseline justify-between gap-x-2.5">
                <p className="text-small font-semibold text-foreground">{t.explanationLabel}</p>
                <span className="text-label text-muted">{t.explanationHint}</span>
              </div>
              <RichTextField
                label={t.explanationLabel}
                size="explanation"
                placeholder={t.explanationPlaceholder}
                value={draft.explanation}
                onChange={(explanation) => setDraft((d) => ({ ...d, explanation }))}
                invalid={!!errors.explanation}
              />
              {errors.explanation && <p className="text-caption text-danger">{errors.explanation}</p>}
            </section>
          </div>

          <aside className="flex flex-col gap-3.5 order-first self-start rounded-tile bg-surface-secondary p-4.5 lg:order-last">
            <p className="text-small font-semibold text-foreground">{t.classificationTitle}</p>
            <SelectInput
              label={t.programLabel}
              required
              options={programOptions}
              selectedKey={draft.programId || null}
              onSelectionChange={(key) =>
                key && setDraft((d) => (d.programId === key ? d : { ...d, programId: key, subjectId: "", chapterId: "" }))
              }
              error={errors.program_id}
            />
            <SelectInput
              label={t.subjectLabel}
              required
              placeholder={subjectOptions.length === 0 ? t.subjectPlaceholderEmpty : t.subjectPlaceholder}
              options={subjectOptions}
              selectedKey={draft.subjectId || null}
              onSelectionChange={(key) => setDraft((d) => ({ ...d, subjectId: key ?? "", chapterId: "" }))}
              isDisabled={subjectOptions.length === 0}
              error={errors.subject_id}
            />
            <SelectInput
              label={t.chapterLabel}
              placeholder={chapterOptions.length === 0 ? t.chapterPlaceholderEmpty : t.chapterPlaceholder}
              options={chapterOptions}
              selectedKey={draft.chapterId || null}
              onSelectionChange={(key) => setDraft((d) => ({ ...d, chapterId: key ?? "" }))}
              isDisabled={chapterOptions.length === 0}
              error={errors.chapter_id}
            />
            <SelectInput
              label={t.difficultyLabel}
              options={difficultyOptions}
              selectedKey={draft.difficulty}
              onSelectionChange={(key) => key && setDraft((d) => ({ ...d, difficulty: key as QuestionDifficulty }))}
              error={errors.difficulty}
            />
            <hr className="border-border" />
            <p className="text-small font-semibold text-foreground">{t.answerTitle}</p>
            <SelectInput
              label={t.typeLabel}
              options={typeOptions}
              selectedKey={draft.type}
              onSelectionChange={(key) => key && setType(key as QuestionType)}
              error={errors.type}
            />
            {draft.type === "pg" && (
              <SwitchInput
                label={t.optionWeightedLabel}
                description={t.optionWeightedDescription}
                isSelected={draft.optionWeighted}
                onChange={(checked) => setDraft((d) => ({ ...d, optionWeighted: checked, correct: [] }))}
              />
            )}
          </aside>
        </div>
      </RichTextGroup>
    </form>
  );
}
