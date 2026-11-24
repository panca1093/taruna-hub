"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@heroui/react";
import { SlidersHorizontal } from "lucide-react";
import { SelectInput } from "@/components/ui/form";
import type { Program, Question, QuestionFilters } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useAdminQuestionFiltersT } from "@/lib/i18n/dict/admin-question-filters";

/** An empty string means "no filter" on that field. */
export interface QuestionFilterValue {
  programId: string;
  subjectId: string;
  chapterId: string;
  type: "" | Question["type"];
  difficulty: "" | Question["difficulty"];
}

export const emptyQuestionFilter: QuestionFilterValue = { programId: "", subjectId: "", chapterId: "", type: "", difficulty: "" };

export function questionFilterQuery(value: QuestionFilterValue): QuestionFilters {
  return {
    ...(value.programId && { program_id: value.programId }),
    ...(value.subjectId && { subject_id: value.subjectId }),
    ...(value.chapterId && { chapter_id: value.chapterId }),
    ...(value.type && { type: value.type }),
    ...(value.difficulty && { difficulty: value.difficulty }),
  };
}

const ALL = "all";

/** The shared question filter: a button beside the search bar that opens a panel of five selects (DESIGN §6.4). */
export function QuestionFilterMenu({
  programs,
  value,
  onChange,
  resetTo = emptyQuestionFilter,
  align = "end",
}: {
  programs: Program[];
  value: QuestionFilterValue;
  onChange: (value: QuestionFilterValue) => void;
  resetTo?: QuestionFilterValue;
  align?: "start" | "end";
}) {
  const t = useAdminQuestionFiltersT();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      // An open select renders its list outside this panel; choosing an option must not close the panel.
      if (document.querySelector('[role="listbox"]')) return;
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !document.querySelector('[role="listbox"]')) setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const named = (items: { id: string; name: string }[], allLabel: string) => [{ id: ALL, label: allLabel }, ...items.map((item) => ({ id: item.id, label: item.name }))];
  const program = programs.find((item) => item.id === draft.programId);
  // Without a program the same subject name repeats across programs, so each one carries its program code.
  const subjects = program ? program.subjects : programs.flatMap((item) => item.subjects.map((subject) => ({ ...subject, name: `${subject.name} (${item.code})` })));
  const chapters = subjects.find((subject) => subject.id === draft.subjectId)?.chapters ?? [];
  const pick = (key: string | null) => (key === null || key === ALL ? "" : key);
  const active = Object.values(value).filter((field) => field !== "").length;

  return (
    <div ref={root} className="relative shrink-0">
      <Button
        variant="secondary"
        aria-expanded={open}
        aria-haspopup="dialog"
        onPress={() => {
          setDraft(value);
          setOpen((current) => !current);
        }}
      >
        <SlidersHorizontal className="size-4" /> {t.filter}
        {active > 0 && <span className="flex size-5 items-center justify-center rounded-full bg-accent text-micro font-semibold text-accent-foreground">{active}</span>}
      </Button>
      {open && (
        <div
          role="dialog"
          aria-label={t.filter}
          className={cn("absolute top-full z-30 mt-2 w-80 max-w-[calc(100vw-2rem)] rounded-menu border border-border bg-overlay p-4 shadow-menu", align === "end" ? "right-0" : "right-0 sm:right-auto sm:left-0")}
        >
          <div className="space-y-3">
            <SelectInput label={t.program} options={named(programs, t.allPrograms)} selectedKey={draft.programId || ALL} onSelectionChange={(key) => setDraft({ ...draft, programId: pick(key), subjectId: "", chapterId: "" })} />
            <SelectInput label={t.subject} options={named(subjects, t.allSubjects)} selectedKey={draft.subjectId || ALL} onSelectionChange={(key) => setDraft({ ...draft, subjectId: pick(key), chapterId: "" })} />
            <SelectInput
              label={t.chapter}
              options={named(chapters, draft.subjectId ? t.allChapters : t.chapterNeedsSubject)}
              selectedKey={draft.chapterId || ALL}
              isDisabled={chapters.length === 0}
              onSelectionChange={(key) => setDraft({ ...draft, chapterId: pick(key) })}
            />
            <div className="grid grid-cols-2 gap-3">
              <SelectInput
                label={t.type}
                options={[{ id: ALL, label: t.all }, ...(["pg", "pg_multi", "isian", "benar_salah"] as const).map((id) => ({ id, label: t.questionType[id] }))]}
                selectedKey={draft.type || ALL}
                onSelectionChange={(key) => setDraft({ ...draft, type: pick(key) as QuestionFilterValue["type"] })}
              />
              <SelectInput
                label={t.difficulty}
                options={[{ id: ALL, label: t.all }, ...(["easy", "medium", "hard"] as const).map((id) => ({ id, label: t.difficultyLevel[id] }))]}
                selectedKey={draft.difficulty || ALL}
                onSelectionChange={(key) => setDraft({ ...draft, difficulty: pick(key) as QuestionFilterValue["difficulty"] })}
              />
            </div>
          </div>
          <div className="mt-4 flex items-center gap-2">
            <button type="button" onClick={() => setDraft(resetTo)} className="mr-auto text-caption font-medium text-muted hover:text-foreground">
              {t.reset}
            </button>
            <Button variant="ghost" size="sm" onPress={() => setOpen(false)}>
              {t.cancel}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onPress={() => {
                onChange(draft);
                setOpen(false);
              }}
            >
              {t.apply}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
