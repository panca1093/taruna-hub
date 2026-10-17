"use client";

import { useMemo, useState } from "react";
import { Button } from "@heroui/react";
import { Plus, Trash2 } from "lucide-react";
import { LatexText } from "@/components/latex";
import { RichTextEditor } from "@/components/admin/rich-text-editor";
import { SelectInput, SwitchInput, TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import type { Program, Question } from "@/lib/api/client";
import { useAdminQuestionEditorT } from "@/lib/i18n/dict/admin-question-editor";

export type QuestionType = Question["type"];
export type QuestionDifficulty = Question["difficulty"];

// A new option carries a placeholder id; the server accepts it in `correct` and answers with real ids.
export interface DraftOption {
  id?: string;
  label: string;
  points?: number;
}

export interface QuestionDraft {
  programId: string;
  subjectId: string;
  chapterId: string;
  type: QuestionType;
  content: string;
  difficulty: QuestionDifficulty;
  explanation: string;
  optionWeighted: boolean;
  caseSensitive: boolean;
  options: DraftOption[];
  correct: string[];
}

const placeholderId = () => "new-" + Math.random().toString(36).slice(2, 10);

export function emptyDraft(programId: string, subjectId: string): QuestionDraft {
  return {
    programId,
    subjectId,
    chapterId: "",
    type: "pg",
    content: "",
    difficulty: "easy",
    explanation: "",
    optionWeighted: false,
    caseSensitive: false,
    options: [1, 2, 3, 4].map(() => ({ id: placeholderId(), label: "" })),
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

const trueFalseOptions: DraftOption[] = [
  { id: "benar", label: "Benar" },
  { id: "salah", label: "Salah" },
];

export function QuestionEditor({
  initial,
  programs,
  errors = {},
  saving = false,
  onSubmit,
}: {
  initial: QuestionDraft;
  programs: Program[];
  errors?: Record<string, string>;
  saving?: boolean;
  onSubmit: (draft: QuestionDraft) => void;
}) {
  const t = useAdminQuestionEditorT();
  const [draft, setDraft] = useState<QuestionDraft>(initial);

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
      setDraft((d) => ({ ...d, type, options: trueFalseOptions.map((option) => ({ ...option })), correct: [], optionWeighted: false }));
    } else if (type === "isian") {
      setDraft((d) => ({ ...d, type, options: [], correct: [], optionWeighted: false }));
    } else {
      setDraft((d) => ({
        ...d,
        type,
        options: d.options.length >= 2 ? d.options : emptyDraft(d.programId, d.subjectId).options,
        correct: [],
        optionWeighted: type === "pg" ? d.optionWeighted : false,
      }));
    }
  };

  const updateOption = (index: number, patch: Partial<DraftOption>) => {
    setDraft((d) => ({ ...d, options: d.options.map((option, i) => (i === index ? { ...option, ...patch } : option)) }));
  };

  const addOption = () => {
    setDraft((d) => ({ ...d, options: [...d.options, { id: placeholderId(), label: "" }] }));
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
      className="flex w-full flex-col gap-4"
    >
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="flex flex-col gap-3">
          <RichTextEditor
            label={t.contentLabel}
            required
            rows={11}
            placeholder={t.contentPlaceholder}
            value={draft.content}
            onChange={(content) => setDraft((d) => ({ ...d, content }))}
          />
          <p className="text-xs text-muted italic">{t.contentHelp}</p>
          {errors.content && <p className="text-xs text-danger">{errors.content}</p>}
          {draft.content && (
            <div className="rounded-lg border border-dashed border-border bg-surface-secondary px-3.5 py-3 text-sm">
              <p className="mb-1 text-[11px] font-medium tracking-wide text-muted uppercase">{t.previewLabel}</p>
              <LatexText text={draft.content} />
            </div>
          )}

          <TextareaInput
            label={t.explanationLabel}
            rows={4}
            placeholder={t.explanationPlaceholder}
            value={draft.explanation}
            onChange={(e) => setDraft((d) => ({ ...d, explanation: e.target.value }))}
            error={errors.explanation}
          />
        </div>

        <div className="flex flex-col gap-4">
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
            label={t.typeLabel}
            options={typeOptions}
            selectedKey={draft.type}
            onSelectionChange={(key) => key && setType(key as QuestionType)}
            error={errors.type}
          />
          <SelectInput
            label={t.difficultyLabel}
            options={difficultyOptions}
            selectedKey={draft.difficulty}
            onSelectionChange={(key) => key && setDraft((d) => ({ ...d, difficulty: key as QuestionDifficulty }))}
            error={errors.difficulty}
          />

          {draft.type === "pg" && (
            <SwitchInput
              label={t.optionWeightedLabel}
              description={t.optionWeightedDescription}
              isSelected={draft.optionWeighted}
              onChange={(checked) => setDraft((d) => ({ ...d, optionWeighted: checked, correct: [] }))}
            />
          )}

          {draft.type === "isian" ? (
            <div className="flex flex-col gap-3">
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
            </div>
          ) : (
            <div className="space-y-2.5">
              <p className="text-[13px] font-medium text-foreground">
                {t.optionsLabel}{" "}
                <span className="text-muted">{draft.optionWeighted ? t.optionsHintWeighted : t.optionsHintMark}</span>
              </p>
              {draft.options.map((option, index) => (
                <div key={option.id ?? `option-${index}`} className="flex flex-col gap-1">
                  <OptionRow
                    option={option}
                    multi={draft.type === "pg_multi"}
                    checked={option.id !== undefined && draft.correct.includes(option.id)}
                    editableLabel={draft.type !== "benar_salah"}
                    weighted={draft.optionWeighted}
                    onToggle={() => toggleCorrect(option)}
                    onChange={(label) => updateOption(index, { label })}
                    onPointsChange={(points) => updateOption(index, { points })}
                    onRemove={
                      draft.type !== "benar_salah" && draft.options.length > 2 ? () => removeOption(index) : undefined
                    }
                    placeholder={t.optionPlaceholder(String.fromCharCode(65 + index))}
                  />
                  {errors[`options[${index}].label`] && (
                    <p className="pl-8 text-xs text-danger">{errors[`options[${index}].label`]}</p>
                  )}
                  {errors[`options[${index}].points`] && (
                    <p className="pl-8 text-xs text-danger">{errors[`options[${index}].points`]}</p>
                  )}
                </div>
              ))}
              {draft.type !== "benar_salah" && draft.options.length < 8 && (
                <Button type="button" variant="ghost" size="sm" onPress={addOption}>
                  <Plus className="size-3.5" /> {t.addOption}
                </Button>
              )}
              {errors.correct && <p className="text-xs text-danger">{errors.correct}</p>}
              {errors.options && <p className="text-xs text-danger">{errors.options}</p>}
              {errors.option_weighted && <p className="text-xs text-danger">{errors.option_weighted}</p>}
            </div>
          )}
          {draft.type === "isian" && errors.options && <p className="text-xs text-danger">{errors.options}</p>}
          {draft.type === "isian" && errors.case_sensitive && <p className="text-xs text-danger">{errors.case_sensitive}</p>}
        </div>
      </div>
    </form>
  );
}

function OptionRow({
  option,
  multi,
  checked,
  editableLabel,
  weighted,
  onToggle,
  onChange,
  onPointsChange,
  onRemove,
  placeholder,
}: {
  option: DraftOption;
  multi: boolean;
  checked: boolean;
  editableLabel: boolean;
  weighted?: boolean;
  onToggle: () => void;
  onChange: (label: string) => void;
  onPointsChange?: (points: number | undefined) => void;
  onRemove?: () => void;
  placeholder: string;
}) {
  return (
    <div className="flex items-center gap-2">
      {weighted ? (
        <input
          type="number"
          min={0}
          max={100}
          step={0.01}
          value={option.points ?? ""}
          placeholder="1"
          onChange={(e) => onPointsChange?.(e.target.value === "" ? undefined : Number(e.target.value))}
          className="w-14 shrink-0 rounded-lg border border-border bg-field px-2 py-2 text-center text-xs font-semibold text-field-foreground outline-none focus:border-accent"
        />
      ) : (
        <button
          type="button"
          onClick={onToggle}
          aria-pressed={checked}
          className={cn(
            "flex size-6 shrink-0 items-center justify-center border-2 text-xs font-bold transition-colors",
            multi ? "rounded-[6px]" : "rounded-full",
            checked ? "border-accent bg-accent text-white" : "border-border text-transparent",
          )}
        >
          ✓
        </button>
      )}
      {editableLabel ? (
        <input
          value={option.label}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="flex-1 rounded-lg border border-border bg-field px-3 py-2 text-sm text-field-foreground outline-none focus:border-accent"
        />
      ) : (
        <span className="flex-1 rounded-lg border border-transparent px-3 py-2 text-sm text-foreground">{option.label}</span>
      )}
      {onRemove && (
        <Button type="button" variant="ghost" size="sm" isIconOnly onPress={onRemove}>
          <Trash2 className="size-3.5 text-muted" />
        </Button>
      )}
    </div>
  );
}
