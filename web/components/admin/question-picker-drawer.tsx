"use client";

import { useCallback, useEffect, useState } from "react";
import { Drawer, Button, toast } from "@heroui/react";
import { Search } from "lucide-react";
import { LatexText } from "@/components/latex";
import { DifficultyBadge, ProgramBadge, QuestionTypeBadge } from "@/components/status-badge";
import { SelectInput } from "@/components/ui/form";
import { api, errorText, type Program, type Question } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useAdminQuestionPickerDrawerT } from "@/lib/i18n/dict/admin-question-picker-drawer";

const PAGE_SIZE = 50;

/** What the builder needs to render an assigned row before and after a save. */
export interface PickedQuestion {
  id: string;
  content: string;
  type: Question["type"];
  difficulty: Question["difficulty"];
  number: number;
  versionId?: string;
}

export function pickedOf(question: Question): PickedQuestion {
  return {
    id: question.id,
    content: question.content,
    type: question.type,
    difficulty: question.difficulty,
    number: question.number,
    versionId: question.version_id,
  };
}

export function QuestionPickerDrawer({
  isOpen,
  onOpenChange,
  programId,
  programs,
  selectedIds,
  onConfirm,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  programId: string;
  programs: Program[];
  selectedIds: string[];
  onConfirm: (ids: string[], picked: PickedQuestion[]) => void;
}) {
  const t = useAdminQuestionPickerDrawerT();
  const [draft, setDraft] = useState<string[]>(selectedIds);
  const [search, setSearch] = useState("");
  const [programFilter, setProgramFilter] = useState(programId === "" ? "all" : programId);
  const [typeFilter, setTypeFilter] = useState<"all" | Question["type"]>("all");
  const [difficultyFilter, setDifficultyFilter] = useState<"all" | Question["difficulty"]>("all");
  const [onlySelected, setOnlySelected] = useState(false);
  const [pool, setPool] = useState<Question[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const programOptions = [{ id: "all", label: t.filterAllProgram }, ...programs.map((program) => ({ id: program.id, label: program.name }))];

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

  const filters = useCallback(
    () => ({
      ...(programFilter === "all" ? {} : { program_id: programFilter }),
      ...(typeFilter === "all" ? {} : { type: typeFilter }),
      ...(difficultyFilter === "all" ? {} : { difficulty: difficultyFilter }),
      ...(search.trim() === "" ? {} : { q: search.trim() }),
    }),
    [programFilter, typeFilter, difficultyFilter, search],
  );

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    api.questions
      .list({ ...filters(), limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setPool(page.questions);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, filters, reloadKey]);

  const loadMore = async () => {
    setLoadingMore(true);
    try {
      const page = await api.questions.list({ ...filters(), limit: PAGE_SIZE, offset: pool?.length ?? 0 });
      setPool((prev) => [...(prev ?? []), ...page.questions]);
      setTotal(page.total);
    } catch (err) {
      toast.danger(t.loadErrorTitle, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setLoadingMore(false);
    }
  };

  const resetOnOpen = (open: boolean) => {
    onOpenChange(open);
    if (open) {
      setDraft(selectedIds);
      setSearch("");
      setProgramFilter(programId === "" ? "all" : programId);
      setTypeFilter("all");
      setDifficultyFilter("all");
      setOnlySelected(false);
      setPool(null);
      setLoadFailed(false);
    }
  };

  // A new filter starts from the loading state; doing it in the handler keeps the effect free of
  // synchronous state updates.
  const refilter = (apply: () => void) => {
    setPool(null);
    setLoadFailed(false);
    apply();
  };

  const visible = (pool ?? []).filter((question) => !onlySelected || draft.includes(question.id));
  const programOf = (id: string) => programs.find((program) => program.id === id);

  return (
    <Drawer isOpen={isOpen} onOpenChange={resetOnOpen}>
      <Drawer.Backdrop>
        <Drawer.Content placement="right">
          <Drawer.Dialog className="flex h-full w-full max-w-2xl flex-col">
            <Drawer.Header className="border-b border-border p-4">
              <div className="flex items-center justify-between">
                <div>
                  <Drawer.Heading className="text-base font-semibold text-foreground">{t.heading}</Drawer.Heading>
                  <p className="mt-0.5 text-xs text-muted">{t.subtitle(draft.length, visible.length, total)}</p>
                </div>
                <div className="flex gap-1.5">
                  <Button variant="ghost" size="sm" onPress={() => setDraft((d) => Array.from(new Set([...d, ...visible.map((q) => q.id)])))}>
                    {t.selectVisible}
                  </Button>
                  <Button variant="ghost" size="sm" className="text-danger" isDisabled={draft.length === 0} onPress={() => setDraft([])}>
                    {t.clear}
                  </Button>
                </div>
              </div>

              <div className="relative mt-3">
                <Search className="absolute top-1/2 left-3 size-3.5 -translate-y-1/2 text-muted" />
                <input
                  value={search}
                  onChange={(e) => refilter(() => setSearch(e.target.value))}
                  placeholder={t.searchPlaceholder}
                  className="w-full rounded-lg border border-border bg-field py-2 pr-3 pl-9 text-sm text-field-foreground outline-none focus:border-accent"
                />
              </div>

              <div className="mt-2.5 flex flex-wrap items-center gap-2">
                <SelectInput
                  className="w-40"
                  options={programOptions}
                  selectedKey={programFilter}
                  onSelectionChange={(key) => refilter(() => setProgramFilter(key ?? "all"))}
                />
                <SelectInput
                  className="w-40"
                  options={typeOptions}
                  selectedKey={typeFilter}
                  onSelectionChange={(key) => refilter(() => setTypeFilter((key ?? "all") as "all" | Question["type"]))}
                />
                <SelectInput
                  className="w-40"
                  options={difficultyOptions}
                  selectedKey={difficultyFilter}
                  onSelectionChange={(key) => refilter(() => setDifficultyFilter((key ?? "all") as "all" | Question["difficulty"]))}
                />
                <button
                  type="button"
                  onClick={() => setOnlySelected((value) => !value)}
                  className={cn(
                    "rounded-full border px-3 py-1.5 text-xs font-medium transition-colors",
                    onlySelected ? "border-accent bg-accent text-accent-foreground" : "border-border text-muted hover:border-accent/40 hover:text-foreground",
                  )}
                >
                  {t.onlySelected}
                </button>
              </div>
            </Drawer.Header>
            <Drawer.Body className="flex-1 space-y-2 overflow-y-auto p-4">
              {loadFailed ? (
                <div className="py-8 text-center">
                  <p className="text-sm text-muted">{t.loadErrorDescription}</p>
                  <Button variant="primary" size="sm" className="mt-3" onPress={() => refilter(() => setReloadKey((key) => key + 1))}>
                    {t.retry}
                  </Button>
                </div>
              ) : pool === null ? (
                <p className="py-8 text-center text-sm text-muted">{t.loading}</p>
              ) : (
                <>
                  {visible.map((question) => {
                    const checked = draft.includes(question.id);
                    const program = programFilter === "all" ? programOf(question.program_id) : undefined;
                    return (
                      <button
                        key={question.id}
                        type="button"
                        onClick={() => setDraft((d) => (checked ? d.filter((id) => id !== question.id) : [...d, question.id]))}
                        className={cn(
                          "flex w-full items-start gap-3 rounded-lg border p-3.5 text-left transition-colors",
                          checked ? "border-accent bg-accent-soft" : "border-border hover:bg-surface-secondary",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-[5px] border-2 text-[10px] font-bold",
                            checked ? "border-accent bg-accent text-white" : "border-border text-transparent",
                          )}
                        >
                          ✓
                        </span>
                        <div className="min-w-0 flex-1">
                          <p className="line-clamp-2 text-sm text-foreground">
                            <LatexText text={question.content} />
                          </p>
                          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                            {program && <ProgramBadge code={program.code} />}
                            <span className="font-mono text-[11px] text-muted">#{question.number}</span>
                            <QuestionTypeBadge type={question.type} />
                            <DifficultyBadge difficulty={question.difficulty} />
                          </div>
                        </div>
                      </button>
                    );
                  })}
                  {visible.length === 0 && <p className="py-8 text-center text-sm text-muted">{t.emptyText}</p>}
                  {pool.length < total && (
                    <div className="flex justify-center pt-2">
                      <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                        {loadingMore ? t.loadingMore : t.loadMore}
                      </Button>
                    </div>
                  )}
                </>
              )}
            </Drawer.Body>
            <Drawer.Footer className="flex justify-end gap-2 border-t border-border p-4">
              <Button variant="ghost" onPress={() => onOpenChange(false)}>
                {t.cancel}
              </Button>
              <Button
                variant="primary"
                onPress={() => {
                  onConfirm(draft, (pool ?? []).map(pickedOf));
                  onOpenChange(false);
                }}
              >
                {t.confirm(draft.length)}
              </Button>
            </Drawer.Footer>
          </Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Backdrop>
    </Drawer>
  );
}
