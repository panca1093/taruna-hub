"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { ArrowDown, ArrowUp, Clock, FileQuestion, GripVertical, Plus, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { LatexText } from "@/components/latex";
import { Panel } from "@/components/page-blocks";
import { SummaryBox } from "@/components/admin/summary-box";
import { DifficultyBadge, QuestionTypeBadge, TryoutStatusBadge } from "@/components/status-badge";
import { SelectInput, SwitchInput, TextInput, TextareaInput } from "@/components/ui/form";
import { QuestionPickerDrawer, type PickedQuestion } from "@/components/admin/question-picker-drawer";
import { api, ApiError, errorText, type Program, type Tryout } from "@/lib/api/client";
import type { ScoringMode } from "@/lib/types";
import { useAdminTryoutBuilderT } from "@/lib/i18n/dict/admin-tryout-builder";

interface SubtestDraft {
  key: string;
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
  graceSeconds: string;
  subtests: SubtestDraft[];
}

const blankDraft: TryoutDraft = {
  title: "",
  programId: "",
  description: "",
  scoringMode: "weighted",
  activeFrom: "",
  activeUntil: "",
  explanationReleaseAt: "",
  randomizeQuestions: false,
  randomizeOptions: false,
  graceSeconds: "0",
  subtests: [],
};

// The date inputs are WIB days; the API stores instants, so both ends of the day are explicit.
function wireDate(date: string, edge: "start" | "end"): string | null {
  if (date === "") return null;
  return `${date}T${edge === "start" ? "00:00:00" : "23:59:59"}+07:00`;
}

function inputDate(instant: string | null): string {
  if (!instant) return "";
  const shifted = new Date(new Date(instant).getTime() + 7 * 60 * 60 * 1000);
  return shifted.toISOString().slice(0, 10);
}

// An empty or fractional field would otherwise reach the API as NaN; the server still enforces 0-3600.
function graceOf(raw: string): number {
  const value = Math.trunc(Number(raw));
  return Number.isFinite(value) ? value : 0;
}

function draftOf(tryout: Tryout): TryoutDraft {
  return {
    title: tryout.title,
    programId: tryout.program_id,
    description: tryout.description,
    scoringMode: tryout.scoring_mode,
    activeFrom: inputDate(tryout.active_from),
    activeUntil: inputDate(tryout.active_until),
    explanationReleaseAt: inputDate(tryout.explanation_release_at),
    randomizeQuestions: tryout.randomize_questions,
    randomizeOptions: tryout.randomize_options,
    graceSeconds: String(tryout.grace_seconds),
    subtests: tryout.subtests.map((subtest) => ({
      key: subtest.id,
      name: subtest.name,
      durationMinutes: Math.round(subtest.duration_seconds / 60),
      questionIds: subtest.questions.map((question) => question.question_id),
    })),
  };
}

function libraryOf(tryout: Tryout): Record<string, PickedQuestion> {
  const out: Record<string, PickedQuestion> = {};
  for (const subtest of tryout.subtests) {
    for (const question of subtest.questions) {
      out[question.question_id] = {
        id: question.question_id,
        content: question.content,
        type: question.type,
        difficulty: question.difficulty,
        number: question.number,
      };
    }
  }
  return out;
}

function frozenOf(tryout: Tryout): Record<string, string> {
  const out: Record<string, string> = {};
  for (const subtest of tryout.subtests) {
    for (const question of subtest.questions) out[question.question_id] = question.version_id;
  }
  return out;
}

export function TryoutBuilder({ tryoutId }: { tryoutId?: string }) {
  const router = useRouter();
  const t = useAdminTryoutBuilderT();
  const isEditing = !!tryoutId;

  const [programs, setPrograms] = useState<Program[] | null>(null);
  const [tryout, setTryout] = useState<Tryout | null>(null);
  const [draft, setDraft] = useState<TryoutDraft>(blankDraft);
  const [library, setLibrary] = useState<Record<string, PickedQuestion>>({});
  const [frozen, setFrozen] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(isEditing);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [saving, setSaving] = useState(false);
  const [working, setWorking] = useState(false);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [pickerFor, setPickerFor] = useState<string | null>(null);

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

  useEffect(() => {
    if (!tryoutId) return;
    let cancelled = false;
    api.tryouts
      .get(tryoutId)
      .then((loaded) => {
        if (cancelled) return;
        setTryout(loaded);
        setDraft(draftOf(loaded));
        setLibrary(libraryOf(loaded));
        setFrozen(frozenOf(loaded));
        setLoading(false);
      })
      .catch(() => {
        if (cancelled) return;
        setLoadFailed(true);
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [tryoutId, reloadKey]);

  const retry = () => {
    setLoading(true);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const locked = tryout?.locked ?? false;
  // A brand-new tryout starts on the first program without writing state from an effect.
  const selectedProgramId = draft.programId !== "" ? draft.programId : (programs?.[0]?.id ?? "");
  const totalQuestions = draft.subtests.reduce((sum, subtest) => sum + subtest.questionIds.length, 0);
  const subtestsError = Object.entries(formErrors).find(([path]) => path.startsWith("subtests"))?.[1];

  const addSubtest = () => {
    setDraft((current) => ({
      ...current,
      subtests: [...current.subtests, { key: `st-new-${Date.now()}`, name: "", durationMinutes: 30, questionIds: [] }],
    }));
  };

  const updateSubtest = (key: string, patch: Partial<SubtestDraft>) => {
    setDraft((current) => ({ ...current, subtests: current.subtests.map((subtest) => (subtest.key === key ? { ...subtest, ...patch } : subtest)) }));
  };

  const removeSubtest = (key: string) => {
    setDraft((current) => ({ ...current, subtests: current.subtests.filter((subtest) => subtest.key !== key) }));
  };

  const moveSubtest = (index: number, direction: -1 | 1) => {
    setDraft((current) => {
      const next = [...current.subtests];
      const target = index + direction;
      if (target < 0 || target >= next.length) return current;
      [next[index], next[target]] = [next[target], next[index]];
      return { ...current, subtests: next };
    });
  };

  const removeQuestion = (key: string, questionId: string) => {
    updateSubtest(key, {
      questionIds: draft.subtests.find((subtest) => subtest.key === key)?.questionIds.filter((id) => id !== questionId) ?? [],
    });
  };

  const bodyOf = () => ({
    program_id: selectedProgramId,
    title: draft.title,
    description: draft.description,
    scoring_mode: draft.scoringMode,
    active_from: wireDate(draft.activeFrom, "start"),
    active_until: wireDate(draft.activeUntil, "end") ?? "",
    explanation_release_at: wireDate(draft.explanationReleaseAt, "end"),
    randomize_questions: draft.randomizeQuestions,
    randomize_options: draft.randomizeOptions,
    grace_seconds: graceOf(draft.graceSeconds),
    subtests: draft.subtests.map((subtest) => ({
      name: subtest.name,
      duration_seconds: Math.round(subtest.durationMinutes * 60),
      questions: subtest.questionIds.map((questionId) => ({
        question_id: questionId,
        ...(frozen[questionId] ? { version_id: frozen[questionId] } : {}),
      })),
    })),
  });

  const absorb = (saved: Tryout) => {
    setTryout(saved);
    setDraft(draftOf(saved));
    setLibrary(libraryOf(saved));
    setFrozen(frozenOf(saved));
  };

  const handleSave = async () => {
    if (tryout && locked) {
      // D10: the grace is the one field Admin may still change once students have started.
      setSaving(true);
      setFormErrors({});
      try {
        const saved = await api.tryouts.setGrace(tryout.id, { grace_seconds: graceOf(draft.graceSeconds) });
        setTryout(saved);
        setDraft((current) => ({ ...current, graceSeconds: String(saved.grace_seconds) }));
        toast.success(t.toastGraceSaved);
      } catch (err) {
        if (err instanceof ApiError && err.fields) setFormErrors(err.fields);
        toast.danger(t.toastSaveFailedTitle, { description: errorText(err, t.loadErrorDescription) });
      } finally {
        setSaving(false);
      }
      return;
    }
    if (!draft.title.trim() || draft.activeUntil === "") {
      toast.danger(t.toastIncompleteTitle, { description: t.toastIncompleteDescription });
      return;
    }
    setSaving(true);
    setFormErrors({});
    try {
      if (tryout) {
        const saved = await api.tryouts.update(tryout.id, { ...bodyOf(), base_revision: tryout.revision });
        absorb(saved);
        toast.success(t.toastUpdated, { description: t.toastSummary(saved.title, saved.subtest_count, saved.question_count) });
      } else {
        const created = await api.tryouts.create(bodyOf());
        toast.success(t.toastCreated, { description: t.toastSummary(created.title, created.subtest_count, created.question_count) });
        router.push("/admin/tryouts");
        return;
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.danger(t.toastConflictTitle, { description: err.message });
      } else {
        if (err instanceof ApiError && err.fields) setFormErrors(err.fields);
        toast.danger(t.toastSaveFailedTitle, { description: errorText(err, t.loadErrorDescription) });
      }
    } finally {
      setSaving(false);
    }
  };

  const changeStatus = async (action: "publish" | "unpublish") => {
    if (!tryout) return;
    setWorking(true);
    setFormErrors({});
    try {
      const saved = action === "publish" ? await api.tryouts.publish(tryout.id) : await api.tryouts.unpublish(tryout.id);
      absorb(saved);
      toast.success(action === "publish" ? t.toastPublished : t.toastDraft);
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFormErrors(err.fields);
      toast.danger(t.toastStatusFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setWorking(false);
    }
  };

  if (loading) {
    return (
      <AppShell title={t.editTitle}>
        <p className="py-14 text-center text-small text-muted">{t.loading}</p>
      </AppShell>
    );
  }

  if (loadFailed) {
    return (
      <AppShell title={t.editTitle}>
        <div className="py-14 text-center">
          <p className="text-small font-medium text-foreground">{t.loadErrorTitle}</p>
          <p className="mt-1 text-small text-muted">{t.loadErrorDescription}</p>
          <Button variant="primary" size="sm" className="mt-4" onPress={retry}>
            {t.retry}
          </Button>
        </div>
      </AppShell>
    );
  }

  const dirty = tryout !== null && JSON.stringify(draft) !== JSON.stringify(draftOf(tryout));
  const graceDirty = tryout !== null && draft.graceSeconds !== String(tryout.grace_seconds);
  const programOptions = (programs ?? []).map((program) => ({ id: program.id, label: program.name }));

  return (
    <AppShell
      title={isEditing ? t.editTitle : t.createTitle}
      description={locked ? t.lockedDescription : undefined}
      breadcrumb={[{ label: t.listCrumb, href: "/admin/tryouts" }, { label: isEditing ? t.editTitle : t.createTitle }]}
      actions={
        <div className="flex gap-2">
          {tryout && (
            <Button
              variant="secondary"
              size="sm"
              isDisabled={locked || saving || working || dirty}
              onPress={() => void changeStatus(tryout.status === "published" ? "unpublish" : "publish")}
            >
              {working ? t.publishing : tryout.status === "published" ? t.unpublish : t.publish}
            </Button>
          )}
          <Button variant="primary" size="sm" onPress={handleSave} isDisabled={(locked && !graceDirty) || saving || working}>
            {saving ? t.saving : t.saveTryout}
          </Button>
        </div>
      }
    >
      <div className="grid items-start gap-6 lg:grid-cols-3">
        <div className="min-w-0 space-y-6 lg:col-span-2">
        {tryout && (
          <div className="flex items-center gap-2 text-caption text-muted">
            <TryoutStatusBadge status={tryout.status} />
            <span>{t.revisionLabel(tryout.revision)}</span>
            {dirty && !locked && <span className="text-warning">{t.saveBeforeStatus}</span>}
          </div>
        )}

        <Panel as="section" className="flex flex-col gap-4">
          <h2 className="font-display text-heading-sm">{t.infoHeading}</h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label={t.titleLabel}
              required
              className="sm:col-span-2"
              value={draft.title}
              error={formErrors.title}
              onChange={(e) => setDraft((current) => ({ ...current, title: e.target.value }))}
              disabled={locked}
            />
            <SelectInput
              label={t.programLabel}
              options={programOptions}
              selectedKey={selectedProgramId}
              error={formErrors.program_id}
              onSelectionChange={(key) => key && setDraft((current) => ({ ...current, programId: key }))}
              isDisabled={locked}
            />
            <TextInput
              label={t.activeFromLabel}
              type="date"
              value={draft.activeFrom}
              error={formErrors.active_from}
              onChange={(e) => setDraft((current) => ({ ...current, activeFrom: e.target.value }))}
              disabled={locked}
            />
            <TextInput
              label={t.activeUntilLabel}
              type="date"
              required
              value={draft.activeUntil}
              error={formErrors.active_until}
              onChange={(e) => setDraft((current) => ({ ...current, activeUntil: e.target.value }))}
              disabled={locked}
            />
          </div>
          <TextareaInput
            label={t.descriptionLabel}
            rows={2}
            value={draft.description}
            error={formErrors.description}
            onChange={(e) => setDraft((current) => ({ ...current, description: e.target.value }))}
            disabled={locked}
          />
          <SwitchInput
            label={t.irtModeLabel}
            description={t.irtModeDescription}
            isSelected={draft.scoringMode === "irt"}
            onChange={(checked) => setDraft((current) => ({ ...current, scoringMode: checked ? "irt" : "weighted" }))}
            isDisabled={locked}
          />
          <SwitchInput
            label={t.holdExplanationLabel}
            description={t.holdExplanationDescription}
            isSelected={!!draft.explanationReleaseAt}
            onChange={(checked) =>
              setDraft((current) => ({ ...current, explanationReleaseAt: checked ? current.activeUntil : "" }))
            }
            isDisabled={locked}
          />
          {draft.explanationReleaseAt && (
            <TextInput
              label={t.explanationReleaseLabel}
              type="date"
              value={draft.explanationReleaseAt}
              error={formErrors.explanation_release_at}
              onChange={(e) => setDraft((current) => ({ ...current, explanationReleaseAt: e.target.value }))}
              disabled={locked}
            />
          )}
          <SwitchInput
            label={t.randomizeQuestionsLabel}
            description={t.randomizeQuestionsDescription}
            isSelected={draft.randomizeQuestions}
            onChange={(checked) => setDraft((current) => ({ ...current, randomizeQuestions: checked }))}
            isDisabled={locked}
          />
          <SwitchInput
            label={t.randomizeOptionsLabel}
            description={t.randomizeOptionsDescription}
            isSelected={draft.randomizeOptions}
            onChange={(checked) => setDraft((current) => ({ ...current, randomizeOptions: checked }))}
            isDisabled={locked}
          />
          <TextInput
            label={t.graceLabel}
            description={t.graceDescription}
            type="number"
            min={0}
            max={3600}
            value={draft.graceSeconds}
            error={formErrors.grace_seconds}
            onChange={(e) => setDraft((current) => ({ ...current, graceSeconds: e.target.value }))}
          />
          <p className="text-caption text-muted">{t.coverColorNote}</p>
        </Panel>

        <Panel as="section" className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4">
            <div>
              <h2 className="font-display text-heading-sm">{t.subtestsHeading}</h2>
              <p className="mt-1 text-caption text-muted">{t.subtestsSubtitle(draft.subtests.length, totalQuestions)}</p>
            </div>
            <Button variant="secondary" size="sm" onPress={addSubtest} isDisabled={locked}>
              <Plus className="size-4" /> {t.addSubtest}
            </Button>
          </div>
          {subtestsError && <p className="text-caption text-danger">{subtestsError}</p>}

          {draft.subtests.length === 0 ? (
            <p className="rounded-control border border-dashed border-border py-8 text-center text-small text-muted">{t.noSubtests}</p>
          ) : (
            <div className="space-y-3">
              {draft.subtests.map((subtest, index) => (
                <div key={subtest.key} className="rounded-tile border border-border bg-background p-4">
                  <div className="flex items-start gap-3">
                    <GripVertical className="mt-2.5 size-4 shrink-0 text-muted" />
                    <div className="grid flex-1 gap-2.5 sm:grid-cols-[1fr_140px]">
                      <TextInput
                        placeholder={t.subtestNamePlaceholder(index + 1)}
                        value={subtest.name}
                        onChange={(e) => updateSubtest(subtest.key, { name: e.target.value })}
                        disabled={locked}
                      />
                      <TextInput
                        type="number"
                        min={1}
                        max={480}
                        placeholder={t.durationPlaceholder}
                        value={String(subtest.durationMinutes)}
                        onChange={(e) => updateSubtest(subtest.key, { durationMinutes: Number(e.target.value) || 0 })}
                        disabled={locked}
                      />
                    </div>
                    <div className="flex shrink-0 flex-col gap-1">
                      <Button variant="ghost" size="sm" isIconOnly isDisabled={locked || index === 0} onPress={() => moveSubtest(index, -1)}>
                        <ArrowUp className="size-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        isIconOnly
                        isDisabled={locked || index === draft.subtests.length - 1}
                        onPress={() => moveSubtest(index, 1)}
                      >
                        <ArrowDown className="size-3.5" />
                      </Button>
                    </div>
                    <Button variant="ghost" size="sm" isIconOnly className="shrink-0 text-danger" isDisabled={locked} onPress={() => removeSubtest(subtest.key)}>
                      <Trash2 className="size-4" />
                    </Button>
                  </div>

                  <div className="mt-3 space-y-1.5 border-t border-border pt-3 pl-7">
                    {subtest.questionIds.length === 0 ? (
                      <p className="text-caption text-muted">{t.emptyQuestions}</p>
                    ) : (
                      subtest.questionIds.map((questionId) => {
                        const question = library[questionId];
                        return (
                          <div key={questionId} className="flex items-center gap-2 rounded-chip bg-surface-secondary px-2.5 py-1.5">
                            <span className="font-mono text-label text-muted">#{question?.number ?? "?"}</span>
                            <span className="line-clamp-1 min-w-0 flex-1 text-caption text-foreground">
                              {question ? <LatexText text={question.content} /> : questionId}
                            </span>
                            {question && <QuestionTypeBadge type={question.type} />}
                            {question && <DifficultyBadge difficulty={question.difficulty} />}
                            <Button
                              variant="ghost"
                              size="sm"
                              isIconOnly
                              className="shrink-0 text-danger"
                              isDisabled={locked}
                              aria-label={t.removeQuestion}
                              onPress={() => removeQuestion(subtest.key, questionId)}
                            >
                              <Trash2 className="size-3" />
                            </Button>
                          </div>
                        );
                      })
                    )}
                  </div>

                  <div className="mt-3 flex items-center justify-between border-t border-border pt-3 pl-7">
                    <span className="flex items-center gap-1.5 text-caption text-muted">
                      <Clock className="size-4" /> {t.durationMinutes(subtest.durationMinutes)}
                      <FileQuestion className="ml-2 size-4" /> {t.questionsSelected(subtest.questionIds.length)}
                    </span>
                    <Button variant="ghost" size="sm" isDisabled={locked} onPress={() => setPickerFor(subtest.key)}>
                      {t.manageQuestions}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
        </div>

        <SummaryBox
          title={t.summaryHeading}
          lines={[
            { label: t.summarySubtests, value: draft.subtests.length },
            { label: t.summaryQuestions, value: totalQuestions },
            { label: t.summaryDuration, value: t.summaryDurationValue(draft.subtests.reduce((sum, subtest) => sum + subtest.durationMinutes, 0)) },
            { label: t.summaryScoring, value: draft.scoringMode === "irt" ? t.summaryScoringIrt : t.summaryScoringWeighted },
          ]}
        />
      </div>

      {pickerFor && (
        <QuestionPickerDrawer
          isOpen={!!pickerFor}
          onOpenChange={(open) => !open && setPickerFor(null)}
          programId={selectedProgramId}
          programs={programs ?? []}
          selectedIds={draft.subtests.find((subtest) => subtest.key === pickerFor)?.questionIds ?? []}
          onConfirm={(ids, picked) => {
            setLibrary((current) => {
              const next = { ...current };
              for (const question of picked) next[question.id] = question;
              return next;
            });
            updateSubtest(pickerFor, { questionIds: ids });
          }}
        />
      )}
    </AppShell>
  );
}
