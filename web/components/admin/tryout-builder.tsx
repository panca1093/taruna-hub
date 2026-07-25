"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { ArrowDown, ArrowUp, Clock, FileQuestion, GripVertical, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { SelectInput, SwitchInput, TextInput, TextareaInput } from "@/components/ui/form";
import { QuestionPickerDrawer } from "@/components/admin/question-picker-drawer";
import { cn } from "@/lib/cn";
import { getSubtestsForTryout, getTryout, programs } from "@/lib/mock-data";
import type { ScoringMode } from "@/lib/types";
import { useAdminTryoutBuilderT } from "@/lib/i18n/dict/admin-tryout-builder";

interface SubtestDraft {
  id: string;
  name: string;
  durationMinutes: number;
  questionIds: string[];
}

interface TryoutDraft {
  title: string;
  programId: string;
  description: string;
  scoringMode: ScoringMode;
  activeFrom: string;
  activeUntil: string;
  explanationReleaseAt: string;
  randomizeQuestions: boolean;
  randomizeOptions: boolean;
  coverFrom: string;
  coverTo: string;
  subtests: SubtestDraft[];
}

const covers = [
  ["#4f46e5", "#8b5cf6"],
  ["#38bdf8", "#6366f1"],
  ["#f59e0b", "#ef4444"],
  ["#10b981", "#0ea5e9"],
  ["#8b5cf6", "#d946ef"],
  ["#0ea5e9", "#4f46e5"],
] as const;

const programOptions = programs.map((p) => ({ id: p.id, label: p.name }));

function draftFromTryout(id: string): TryoutDraft | null {
  const tryout = getTryout(id);
  if (!tryout) return null;
  return {
    title: tryout.title,
    programId: tryout.programId,
    description: tryout.description,
    scoringMode: tryout.scoringMode,
    activeFrom: tryout.activeFrom ?? "",
    activeUntil: tryout.activeUntil,
    explanationReleaseAt: tryout.explanationReleaseAt ?? "",
    randomizeQuestions: tryout.randomizeQuestions ?? false,
    randomizeOptions: tryout.randomizeOptions ?? false,
    coverFrom: tryout.coverFrom,
    coverTo: tryout.coverTo,
    subtests: getSubtestsForTryout(id).map((s) => ({
      id: s.id,
      name: s.name,
      durationMinutes: Math.round(s.durationSeconds / 60),
      questionIds: s.questionIds,
    })),
  };
}

const blankDraft: TryoutDraft = {
  title: "",
  programId: programs[0].id,
  description: "",
  scoringMode: "weighted",
  activeFrom: "",
  activeUntil: "",
  explanationReleaseAt: "",
  randomizeQuestions: false,
  randomizeOptions: false,
  coverFrom: covers[0][0],
  coverTo: covers[0][1],
  subtests: [],
};

export function TryoutBuilder({ tryoutId }: { tryoutId?: string }) {
  const router = useRouter();
  const t = useAdminTryoutBuilderT();
  const isEditing = !!tryoutId;
  const locked = isEditing ? (getTryout(tryoutId!)?.participantCount ?? 0) > 0 : false;
  const [draft, setDraft] = useState<TryoutDraft>(() => (tryoutId ? (draftFromTryout(tryoutId) ?? blankDraft) : blankDraft));
  const [pickerFor, setPickerFor] = useState<string | null>(null);

  const addSubtest = () => {
    setDraft((d) => ({
      ...d,
      subtests: [...d.subtests, { id: `st-new-${Date.now()}`, name: "", durationMinutes: 30, questionIds: [] }],
    }));
  };

  const updateSubtest = (id: string, patch: Partial<SubtestDraft>) => {
    setDraft((d) => ({ ...d, subtests: d.subtests.map((s) => (s.id === id ? { ...s, ...patch } : s)) }));
  };

  const removeSubtest = (id: string) => {
    setDraft((d) => ({ ...d, subtests: d.subtests.filter((s) => s.id !== id) }));
  };

  const moveSubtest = (index: number, dir: -1 | 1) => {
    setDraft((d) => {
      const next = [...d.subtests];
      const target = index + dir;
      if (target < 0 || target >= next.length) return d;
      [next[index], next[target]] = [next[target], next[index]];
      return { ...d, subtests: next };
    });
  };

  const totalQuestions = draft.subtests.reduce((s, st) => s + st.questionIds.length, 0);

  const handleSave = () => {
    if (!draft.title || draft.subtests.length === 0) {
      toast.danger(t.toastIncompleteTitle, { description: t.toastIncompleteDescription });
      return;
    }
    toast.success(isEditing ? t.toastUpdated : t.toastCreated, {
      description: t.toastSummary(draft.title, draft.subtests.length, totalQuestions),
    });
    router.push("/admin/tryouts");
  };

  return (
    <AppShell
      title={isEditing ? t.editTitle : t.createTitle}
      description={locked ? t.lockedDescription : undefined}
      actions={
        <Button variant="primary" size="sm" onPress={handleSave} isDisabled={locked}>
          {t.saveTryout}
        </Button>
      }
    >
      <div className="mx-auto max-w-3xl space-y-6">
        <Card className="gap-4 p-5">
          <p className="text-sm font-semibold text-foreground">{t.infoHeading}</p>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label={t.titleLabel}
              required
              className="sm:col-span-2"
              value={draft.title}
              onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
              disabled={locked}
            />
            <SelectInput
              label={t.programLabel}
              options={programOptions}
              selectedKey={draft.programId}
              onSelectionChange={(k) => k && setDraft((d) => ({ ...d, programId: k }))}
              isDisabled={locked}
            />
            <TextInput
              label={t.activeFromLabel}
              type="date"
              value={draft.activeFrom}
              onChange={(e) => setDraft((d) => ({ ...d, activeFrom: e.target.value }))}
              disabled={locked}
            />
            <TextInput
              label={t.activeUntilLabel}
              type="date"
              required
              value={draft.activeUntil}
              onChange={(e) => setDraft((d) => ({ ...d, activeUntil: e.target.value }))}
              disabled={locked}
            />
          </div>
          <TextareaInput
            label={t.descriptionLabel}
            rows={2}
            value={draft.description}
            onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))}
            disabled={locked}
          />
          <SwitchInput
            label={t.irtModeLabel}
            description={t.irtModeDescription}
            isSelected={draft.scoringMode === "irt"}
            onChange={(checked) => setDraft((d) => ({ ...d, scoringMode: checked ? "irt" : "weighted" }))}
            isDisabled={locked}
          />
          <SwitchInput
            label={t.holdExplanationLabel}
            description={t.holdExplanationDescription}
            isSelected={!!draft.explanationReleaseAt}
            onChange={(checked) => setDraft((d) => ({ ...d, explanationReleaseAt: checked ? d.activeUntil : "" }))}
            isDisabled={locked}
          />
          {draft.explanationReleaseAt && (
            <TextInput
              label={t.explanationReleaseLabel}
              type="date"
              value={draft.explanationReleaseAt}
              onChange={(e) => setDraft((d) => ({ ...d, explanationReleaseAt: e.target.value }))}
              disabled={locked}
            />
          )}
          <SwitchInput
            label={t.randomizeQuestionsLabel}
            description={t.randomizeQuestionsDescription}
            isSelected={draft.randomizeQuestions}
            onChange={(checked) => setDraft((d) => ({ ...d, randomizeQuestions: checked }))}
            isDisabled={locked}
          />
          <SwitchInput
            label={t.randomizeOptionsLabel}
            description={t.randomizeOptionsDescription}
            isSelected={draft.randomizeOptions}
            onChange={(checked) => setDraft((d) => ({ ...d, randomizeOptions: checked }))}
            isDisabled={locked}
          />
          <div>
            <p className="mb-2 text-[13px] font-medium text-foreground">{t.coverColorLabel}</p>
            <div className="flex gap-2">
              {covers.map(([from, to]) => (
                <button
                  key={from + to}
                  type="button"
                  disabled={locked}
                  onClick={() => setDraft((d) => ({ ...d, coverFrom: from, coverTo: to }))}
                  className={cn(
                    "size-8 rounded-md ring-offset-2 ring-offset-surface transition-shadow disabled:cursor-not-allowed disabled:opacity-40",
                    draft.coverFrom === from && draft.coverTo === to && "ring-2 ring-accent",
                  )}
                  style={{ background: `linear-gradient(135deg, ${from}, ${to})` }}
                  aria-label={t.coverColorAria}
                />
              ))}
            </div>
          </div>
        </Card>

        <Card className="gap-4 p-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold text-foreground">{t.subtestsHeading}</p>
              <p className="text-xs text-muted">{t.subtestsSubtitle(draft.subtests.length, totalQuestions)}</p>
            </div>
            <Button variant="secondary" size="sm" onPress={addSubtest} isDisabled={locked}>
              <Plus className="size-3.5" /> {t.addSubtest}
            </Button>
          </div>

          {draft.subtests.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted">
              {t.noSubtests}
            </p>
          ) : (
            <div className="space-y-3">
              {draft.subtests.map((s, i) => (
                <div key={s.id} className="rounded-lg border border-border p-3.5">
                  <div className="flex items-start gap-3">
                    <GripVertical className="mt-2.5 size-4 shrink-0 text-muted" />
                    <div className="grid flex-1 gap-2.5 sm:grid-cols-[1fr_140px]">
                      <TextInput
                        placeholder={t.subtestNamePlaceholder(i + 1)}
                        value={s.name}
                        onChange={(e) => updateSubtest(s.id, { name: e.target.value })}
                        disabled={locked}
                      />
                      <TextInput
                        type="number"
                        min={5}
                        placeholder={t.durationPlaceholder}
                        value={String(s.durationMinutes)}
                        onChange={(e) => updateSubtest(s.id, { durationMinutes: Number(e.target.value) || 0 })}
                        disabled={locked}
                      />
                    </div>
                    <div className="flex shrink-0 flex-col gap-1">
                      <Button variant="ghost" size="sm" isIconOnly isDisabled={locked || i === 0} onPress={() => moveSubtest(i, -1)}>
                        <ArrowUp className="size-3.5" />
                      </Button>
                      <Button variant="ghost" size="sm" isIconOnly isDisabled={locked || i === draft.subtests.length - 1} onPress={() => moveSubtest(i, 1)}>
                        <ArrowDown className="size-3.5" />
                      </Button>
                    </div>
                    <Button variant="ghost" size="sm" isIconOnly className="shrink-0 text-danger" isDisabled={locked} onPress={() => removeSubtest(s.id)}>
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                  <div className="mt-3 flex items-center justify-between border-t border-border pt-3 pl-7">
                    <span className="flex items-center gap-1.5 text-xs text-muted">
                      <Clock className="size-3.5" /> {t.durationMinutes(s.durationMinutes)}
                      <FileQuestion className="ml-2 size-3.5" /> {t.questionsSelected(s.questionIds.length)}
                    </span>
                    <Button variant="ghost" size="sm" isDisabled={locked} onPress={() => setPickerFor(s.id)}>
                      {t.manageQuestions}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>

      {pickerFor && (
        <QuestionPickerDrawer
          isOpen={!!pickerFor}
          onOpenChange={(open) => !open && setPickerFor(null)}
          programId={draft.programId}
          selectedIds={draft.subtests.find((s) => s.id === pickerFor)?.questionIds ?? []}
          onConfirm={(ids) => updateSubtest(pickerFor, { questionIds: ids })}
        />
      )}
    </AppShell>
  );
}
