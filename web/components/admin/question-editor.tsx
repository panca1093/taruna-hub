"use client";

import { useMemo, useState } from "react";
import { Button } from "@heroui/react";
import { Plus, Trash2 } from "lucide-react";
import { LatexText } from "@/components/latex";
import { RichTextEditor } from "@/components/admin/rich-text-editor";
import { SelectInput, SwitchInput, TextInput, TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { getProgram, programs } from "@/lib/mock-data";
import type { Difficulty, Question, QuestionOption, QuestionType } from "@/lib/types";
import { useAdminQuestionEditorT } from "@/lib/i18n/dict/admin-question-editor";

export type QuestionDraft = Omit<Question, "id">;

const weightByDifficulty: Record<Difficulty, number> = { easy: 1, medium: 2, hard: 3 };

export function emptyDraft(programId: string): QuestionDraft {
  return {
    programId,
    subject: "",
    chapter: "",
    type: "pg",
    content: "",
    options: [
      { id: "a", label: "" },
      { id: "b", label: "" },
      { id: "c", label: "" },
      { id: "d", label: "" },
    ],
    correctAnswer: [],
    difficulty: "easy",
    weight: 1,
    explanation: "",
    optionWeighted: false,
  };
}

const programOptions = programs.map((p) => ({ id: p.id, label: p.name }));

export function QuestionEditor({
  initial,
  onSubmit,
}: {
  initial: QuestionDraft;
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

  const program = getProgram(draft.programId);
  const subjectOptions = useMemo(
    () => (program?.subjects ?? []).map((s) => ({ id: s.id, label: s.name })),
    [program],
  );
  const activeSubject = program?.subjects.find((s) => s.name === draft.subject);
  const chapterOptions = useMemo(
    () => (activeSubject?.chapters ?? []).map((c) => ({ id: c, label: c })),
    [activeSubject],
  );

  const setType = (type: QuestionType) => {
    if (type === "benar_salah") {
      setDraft((d) => ({
        ...d,
        type,
        options: [
          { id: "benar", label: "Benar" },
          { id: "salah", label: "Salah" },
        ],
        correctAnswer: [],
        optionWeighted: false,
      }));
    } else if (type === "isian") {
      setDraft((d) => ({ ...d, type, options: undefined, correctAnswer: [], optionWeighted: false }));
    } else {
      setDraft((d) => ({
        ...d,
        type,
        options: d.options && d.options.length >= 2 ? d.options : emptyDraft(d.programId).options,
        correctAnswer: [],
        optionWeighted: type === "pg" ? d.optionWeighted : false,
      }));
    }
  };

  const updateOption = (id: string, label: string) => {
    setDraft((d) => ({ ...d, options: d.options?.map((o) => (o.id === id ? { ...o, label } : o)) }));
  };

  const updateOptionPoints = (id: string, points: number | undefined) => {
    setDraft((d) => ({ ...d, options: d.options?.map((o) => (o.id === id ? { ...o, points } : o)) }));
  };

  const addOption = () => {
    setDraft((d) => {
      const nextId = String.fromCharCode(97 + (d.options?.length ?? 0));
      return { ...d, options: [...(d.options ?? []), { id: nextId, label: "" }] };
    });
  };

  const removeOption = (id: string) => {
    setDraft((d) => ({
      ...d,
      options: d.options?.filter((o) => o.id !== id),
      correctAnswer: d.correctAnswer.filter((a) => a !== id),
    }));
  };

  const toggleCorrect = (id: string) => {
    setDraft((d) => {
      if (d.type === "pg_multi") {
        const has = d.correctAnswer.includes(id);
        return { ...d, correctAnswer: has ? d.correctAnswer.filter((a) => a !== id) : [...d.correctAnswer, id] };
      }
      return { ...d, correctAnswer: [id] };
    });
  };

  return (
    <form
      id="question-editor-form"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(draft);
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
          {draft.content && (
            <div className="rounded-lg border border-dashed border-border bg-surface-secondary px-3.5 py-3 text-sm">
              <p className="mb-1 text-[11px] font-medium tracking-wide text-muted uppercase">{t.previewLabel}</p>
              <LatexText text={draft.content} />
            </div>
          )}

          <TextareaInput
            label={t.explanationLabel}
            required
            rows={4}
            placeholder={t.explanationPlaceholder}
            value={draft.explanation}
            onChange={(e) => setDraft((d) => ({ ...d, explanation: e.target.value }))}
          />
        </div>

        <div className="flex flex-col gap-4">
          <SelectInput
            label={t.programLabel}
            required
            options={programOptions}
            selectedKey={draft.programId}
            onSelectionChange={(k) => k && setDraft((d) => (d.programId === k ? d : { ...d, programId: k, subject: "", chapter: "" }))}
          />
          <SelectInput
            label={t.subjectLabel}
            required
            placeholder={subjectOptions.length === 0 ? t.subjectPlaceholderEmpty : t.subjectPlaceholder}
            options={subjectOptions}
            selectedKey={activeSubject?.id ?? null}
            onSelectionChange={(k) => {
              const s = program?.subjects.find((x) => x.id === k);
              setDraft((d) => ({ ...d, subject: s?.name ?? "", chapter: "" }));
            }}
            isDisabled={subjectOptions.length === 0}
          />
          <SelectInput
            label={t.chapterLabel}
            placeholder={chapterOptions.length === 0 ? t.chapterPlaceholderEmpty : t.chapterPlaceholder}
            options={chapterOptions}
            selectedKey={draft.chapter || null}
            onSelectionChange={(k) => setDraft((d) => ({ ...d, chapter: k ?? "" }))}
            isDisabled={chapterOptions.length === 0}
          />
          <SelectInput label={t.typeLabel} options={typeOptions} selectedKey={draft.type} onSelectionChange={(k) => k && setType(k as QuestionType)} />
          <SelectInput
            label={t.difficultyLabel}
            options={difficultyOptions}
            selectedKey={draft.difficulty}
            onSelectionChange={(k) => {
              const difficulty = (k as Difficulty) ?? "easy";
              setDraft((d) => ({ ...d, difficulty, weight: weightByDifficulty[difficulty] }));
            }}
          />

          {draft.type === "pg" && (
            <SwitchInput
              label={t.optionWeightedLabel}
              description={t.optionWeightedDescription}
              isSelected={!!draft.optionWeighted}
              onChange={(checked) => setDraft((d) => ({ ...d, optionWeighted: checked, correctAnswer: [] }))}
            />
          )}

          {draft.type === "isian" ? (
            <TextInput
              label={t.correctAnswerLabel}
              description={t.correctAnswerDescription}
              required
              value={draft.correctAnswer[0] ?? ""}
              onChange={(e) => setDraft((d) => ({ ...d, correctAnswer: [e.target.value] }))}
            />
          ) : (
            <div className="space-y-2.5">
              <p className="text-[13px] font-medium text-foreground">
                {t.optionsLabel}{" "}
                <span className="text-muted">{draft.optionWeighted ? t.optionsHintWeighted : t.optionsHintMark}</span>
              </p>
              {draft.options?.map((opt) => (
                <OptionRow
                  key={opt.id}
                  option={opt}
                  multi={draft.type === "pg_multi"}
                  checked={draft.correctAnswer.includes(opt.id)}
                  editableLabel={draft.type !== "benar_salah"}
                  weighted={draft.optionWeighted}
                  onToggle={() => toggleCorrect(opt.id)}
                  onChange={(label) => updateOption(opt.id, label)}
                  onPointsChange={(points) => updateOptionPoints(opt.id, points)}
                  onRemove={draft.type !== "benar_salah" && (draft.options?.length ?? 0) > 2 ? () => removeOption(opt.id) : undefined}
                  optionPlaceholder={t.optionPlaceholder}
                />
              ))}
              {draft.type !== "benar_salah" && (
                <Button type="button" variant="ghost" size="sm" onPress={addOption}>
                  <Plus className="size-3.5" /> {t.addOption}
                </Button>
              )}
            </div>
          )}
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
  optionPlaceholder,
}: {
  option: QuestionOption;
  multi: boolean;
  checked: boolean;
  editableLabel: boolean;
  weighted?: boolean;
  onToggle: () => void;
  onChange: (label: string) => void;
  onPointsChange?: (points: number | undefined) => void;
  onRemove?: () => void;
  optionPlaceholder: (label: string) => string;
}) {
  return (
    <div className="flex items-center gap-2">
      {weighted ? (
        <input
          type="number"
          min={0}
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
          placeholder={optionPlaceholder(option.id.toUpperCase())}
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
