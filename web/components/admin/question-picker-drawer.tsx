"use client";

import { useCallback, useEffect, useState } from "react";
import { Drawer, Button, toast } from "@heroui/react";
import { Search } from "lucide-react";
import { emptyQuestionFilter, QuestionFilterMenu, questionFilterQuery, type QuestionFilterValue } from "@/components/admin/question-filter-menu";
import { api, errorText, type Program, type Question } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { RichDocSummary } from "@/components/rich-doc";
import { useAdminQuestionPickerDrawerT } from "@/lib/i18n/dict/admin-question-picker-drawer";

const PAGE_SIZE = 50;

/** What the builder needs to render an assigned row before and after a save. */
export interface PickedQuestion {
  id: string;
  content: Question["content"];
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
  const initialFilter: QuestionFilterValue = { ...emptyQuestionFilter, programId };
  const [filter, setFilter] = useState(initialFilter);
  const [onlySelected, setOnlySelected] = useState(false);
  const [pool, setPool] = useState<Question[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const filters = useCallback(() => ({ ...questionFilterQuery(filter), ...(search.trim() === "" ? {} : { q: search.trim() }) }), [filter, search]);

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
      setFilter(initialFilter);
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
  const subjects = programs.flatMap((program) => program.subjects);
  const topicOf = (question: Question) => {
    const subject = subjects.find((item) => item.id === question.subject_id);
    const chapter = subject?.chapters.find((item) => item.id === question.chapter_id);
    return [subject?.name, chapter?.name].filter(Boolean).join(" › ");
  };

  return (
    <Drawer isOpen={isOpen} onOpenChange={resetOnOpen}>
      <Drawer.Backdrop>
        <Drawer.Content placement="right">
          <Drawer.Dialog className="flex h-full w-full max-w-2xl flex-col">
            <Drawer.Header className="px-5 pt-5 pb-3.5">
              <div className="flex items-baseline justify-between gap-3">
                <Drawer.Heading className="font-display text-heading-sm text-foreground">{t.heading}</Drawer.Heading>
                <p className="text-caption text-muted">{pool === null ? "" : t.matching(total)}</p>
              </div>

              <div className="mt-3 flex items-center gap-2">
                <div className="relative min-w-0 flex-1">
                  <Search className="absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted" />
                  <input
                    value={search}
                    onChange={(e) => refilter(() => setSearch(e.target.value))}
                    placeholder={t.searchPlaceholder}
                    aria-label={t.searchPlaceholder}
                    className="w-full rounded-full border border-border bg-field py-2 pr-4 pl-10 text-small text-field-foreground outline-none focus:border-accent"
                  />
                </div>
                <QuestionFilterMenu programs={programs} value={filter} resetTo={initialFilter} onChange={(next) => refilter(() => setFilter(next))} />
              </div>
            </Drawer.Header>

            <div className="flex flex-wrap items-center justify-between gap-2 border-y border-border px-5 py-2">
              <div className="inline-flex rounded-full bg-surface-secondary p-0.5">
                {([false, true] as const).map((only) => (
                  <button
                    key={String(only)}
                    type="button"
                    aria-pressed={onlySelected === only}
                    onClick={() => setOnlySelected(only)}
                    className={cn(
                      "rounded-full px-3 py-1 text-caption font-medium transition-colors",
                      onlySelected === only ? "bg-surface text-foreground ring-1 ring-border" : "text-muted hover:text-foreground",
                    )}
                  >
                    {only ? t.showSelected(draft.length) : t.showAll}
                  </button>
                ))}
              </div>
              <div className="flex gap-1">
                <Button variant="ghost" size="sm" onPress={() => setDraft((d) => Array.from(new Set([...d, ...visible.map((q) => q.id)])))}>
                  {t.selectVisible}
                </Button>
                <Button variant="ghost" size="sm" isDisabled={draft.length === 0} onPress={() => setDraft([])}>
                  {t.clear}
                </Button>
              </div>
            </div>

            <Drawer.Body className="flex-1 overflow-y-auto px-5 py-0">
              {loadFailed ? (
                <div className="py-8 text-center">
                  <p className="text-small text-muted">{t.loadErrorDescription}</p>
                  <Button variant="primary" size="sm" className="mt-3" onPress={() => refilter(() => setReloadKey((key) => key + 1))}>
                    {t.retry}
                  </Button>
                </div>
              ) : pool === null ? (
                <p className="py-8 text-center text-small text-muted">{t.loading}</p>
              ) : (
                <>
                  {visible.map((question) => {
                    const checked = draft.includes(question.id);
                    const meta = [`#${question.number}`, topicOf(question), t.questionType[question.type], t.difficulty[question.difficulty]].filter(Boolean);
                    return (
                      <label key={question.id} className="flex cursor-pointer items-start gap-3 border-b border-border py-3 last:border-b-0">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => setDraft((d) => (checked ? d.filter((id) => id !== question.id) : [...d, question.id]))}
                          className="mt-0.5 size-4.5 shrink-0 accent-accent"
                        />
                        <span className="min-w-0 flex-1">
                          <span className="line-clamp-2 text-small text-foreground">
                            <RichDocSummary doc={question.content} />
                          </span>
                          <span className="mt-0.5 block text-caption text-muted">{meta.join(" · ")}</span>
                        </span>
                      </label>
                    );
                  })}
                  {visible.length === 0 && <p className="py-8 text-center text-small text-muted">{t.emptyText}</p>}
                  {pool.length < total && (
                    <div className="flex justify-center py-3">
                      <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                        {loadingMore ? t.loadingMore : t.loadMore}
                      </Button>
                    </div>
                  )}
                </>
              )}
            </Drawer.Body>
            <Drawer.Footer className="flex items-center gap-2 border-t border-border px-5 py-3.5">
              <p className="mr-auto text-caption text-muted">{t.selectedCount(draft.length)}</p>
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
