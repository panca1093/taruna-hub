"use client";

import { useMemo, useState } from "react";
import { Drawer, Button } from "@heroui/react";
import { Search } from "lucide-react";
import { LatexText } from "@/components/latex";
import { DifficultyBadge, ProgramBadge, QuestionTypeBadge } from "@/components/status-badge";
import { SelectInput } from "@/components/ui/form";
import { getProgram, getQuestionsByProgram, programs, questions } from "@/lib/mock-data";
import { cn } from "@/lib/cn";
import { useAdminQuestionPickerDrawerT } from "@/lib/i18n/dict/admin-question-picker-drawer";

export function QuestionPickerDrawer({
  isOpen,
  onOpenChange,
  programId,
  selectedIds,
  onConfirm,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  programId: string;
  selectedIds: string[];
  onConfirm: (ids: string[]) => void;
}) {
  const t = useAdminQuestionPickerDrawerT();
  const [draft, setDraft] = useState<string[]>(selectedIds);
  const [search, setSearch] = useState("");
  const [programFilter, setProgramFilter] = useState(programId);
  const [subjectFilter, setSubjectFilter] = useState("all");
  const [typeFilter, setTypeFilter] = useState("all");
  const [difficultyFilter, setDifficultyFilter] = useState("all");
  const [onlySelected, setOnlySelected] = useState(false);

  const programOptions = [{ id: "all", label: t.filterAllProgram }, ...programs.map((p) => ({ id: p.id, label: p.name }))];

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

  const pool = useMemo(
    () => (programFilter === "all" ? questions : getQuestionsByProgram(programFilter)),
    [programFilter],
  );

  const subjectOptions = useMemo(() => {
    const subjects =
      programFilter === "all"
        ? Array.from(new Set(programs.flatMap((p) => p.subjects.map((s) => s.name)))).sort()
        : (getProgram(programFilter)?.subjects ?? []).map((s) => s.name);
    return [{ id: "all", label: t.filterAllSubject }, ...subjects.map((s) => ({ id: s, label: s }))];
  }, [programFilter, t.filterAllSubject]);

  const resetOnOpen = (open: boolean) => {
    onOpenChange(open);
    if (open) {
      setDraft(selectedIds);
      setSearch("");
      setProgramFilter(programId);
      setSubjectFilter("all");
      setTypeFilter("all");
      setDifficultyFilter("all");
      setOnlySelected(false);
    }
  };

  const filtered = pool.filter((q) => {
    if (onlySelected && !draft.includes(q.id)) return false;
    if (subjectFilter !== "all" && q.subject !== subjectFilter) return false;
    if (typeFilter !== "all" && q.type !== typeFilter) return false;
    if (difficultyFilter !== "all" && q.difficulty !== difficultyFilter) return false;
    if (search && !q.content.toLowerCase().includes(search.toLowerCase()) && !q.subject.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  return (
    <Drawer isOpen={isOpen} onOpenChange={resetOnOpen}>
      <Drawer.Backdrop>
        <Drawer.Content placement="right">
          <Drawer.Dialog className="flex h-full w-full max-w-2xl flex-col">
            <Drawer.Header className="border-b border-border p-4">
              <div className="flex items-center justify-between">
                <div>
                  <Drawer.Heading className="text-base font-semibold text-foreground">{t.heading}</Drawer.Heading>
                  <p className="mt-0.5 text-xs text-muted">{t.subtitle(draft.length, filtered.length, pool.length)}</p>
                </div>
                <div className="flex gap-1.5">
                  <Button variant="ghost" size="sm" onPress={() => setDraft((d) => Array.from(new Set([...d, ...filtered.map((q) => q.id)])))}>
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
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={t.searchPlaceholder}
                  className="w-full rounded-lg border border-border bg-field py-2 pr-3 pl-9 text-sm text-field-foreground outline-none focus:border-accent"
                />
              </div>

              <div className="mt-2.5 flex flex-wrap items-center gap-2">
                <SelectInput
                  className="w-40"
                  options={programOptions}
                  selectedKey={programFilter}
                  onSelectionChange={(k) => {
                    setProgramFilter(k ?? "all");
                    setSubjectFilter("all");
                  }}
                />
                <SelectInput className="w-40" options={subjectOptions} selectedKey={subjectFilter} onSelectionChange={(k) => setSubjectFilter(k ?? "all")} />
                <SelectInput className="w-40" options={typeOptions} selectedKey={typeFilter} onSelectionChange={(k) => setTypeFilter(k ?? "all")} />
                <SelectInput className="w-40" options={difficultyOptions} selectedKey={difficultyFilter} onSelectionChange={(k) => setDifficultyFilter(k ?? "all")} />
                <button
                  type="button"
                  onClick={() => setOnlySelected((v) => !v)}
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
              {filtered.map((q) => {
                const checked = draft.includes(q.id);
                return (
                  <button
                    key={q.id}
                    type="button"
                    onClick={() => setDraft((d) => (checked ? d.filter((id) => id !== q.id) : [...d, q.id]))}
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
                        <LatexText text={q.content} />
                      </p>
                      <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                        {programFilter === "all" && (() => {
                          const program = getProgram(q.programId);
                          return program ? <ProgramBadge code={program.code} /> : null;
                        })()}
                        <span className="text-xs font-medium text-muted">{q.subject}</span>
                        <QuestionTypeBadge type={q.type} />
                        <DifficultyBadge difficulty={q.difficulty} />
                        <span className="font-mono text-[11px] text-muted">{t.weightLabel(q.weight)}</span>
                      </div>
                    </div>
                  </button>
                );
              })}
              {filtered.length === 0 && (
                <p className="py-8 text-center text-sm text-muted">{t.emptyText}</p>
              )}
            </Drawer.Body>
            <Drawer.Footer className="flex justify-end gap-2 border-t border-border p-4">
              <Button variant="ghost" onPress={() => onOpenChange(false)}>
                {t.cancel}
              </Button>
              <Button
                variant="primary"
                onPress={() => {
                  onConfirm(draft);
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
