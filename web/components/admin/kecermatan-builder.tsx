"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { AppShell } from "@/components/app-shell";
import { SelectInput, TextInput, TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import { getKecermatanTest, programs } from "@/lib/mock-data";
import type { KecermatanItemType } from "@/lib/types";
import { useAdminKecermatanBuilderT } from "@/lib/i18n/dict/admin-kecermatan-builder";

interface KecermatanDraft {
  title: string;
  programId: string;
  description: string;
  itemType: KecermatanItemType;
  itemLength: number;
  optionsPerSoal: number;
  soalPerKolom: number;
  kolomCount: number;
  kolomDurationMinutes: number;
  coverFrom: string;
  coverTo: string;
}

const covers = [
  ["#f59e0b", "#ef4444"],
  ["#0ea5e9", "#4f46e5"],
  ["#10b981", "#0ea5e9"],
  ["#8b5cf6", "#d946ef"],
  ["#4f46e5", "#8b5cf6"],
  ["#f97316", "#f43f5e"],
] as const;

const programOptions = programs.map((p) => ({ id: p.id, label: p.name }));

function draftFromTest(id: string): KecermatanDraft | null {
  const test = getKecermatanTest(id);
  if (!test) return null;
  return {
    title: test.title,
    programId: test.programId,
    description: test.description,
    itemType: test.itemType,
    itemLength: test.itemLength,
    optionsPerSoal: test.optionsPerSoal,
    soalPerKolom: test.soalPerKolom,
    kolomCount: test.kolomCount,
    kolomDurationMinutes: Math.round(test.kolomDurationSeconds / 60),
    coverFrom: test.coverFrom,
    coverTo: test.coverTo,
  };
}

const blankDraft: KecermatanDraft = {
  title: "",
  programId: programs[0].id,
  description: "",
  itemType: "char",
  itemLength: 4,
  optionsPerSoal: 5,
  soalPerKolom: 8,
  kolomCount: 3,
  kolomDurationMinutes: 2,
  coverFrom: covers[0][0],
  coverTo: covers[0][1],
};

export function KecermatanBuilder({ testId }: { testId?: string }) {
  const router = useRouter();
  const t = useAdminKecermatanBuilderT();
  const isEditing = !!testId;
  const locked = isEditing ? (getKecermatanTest(testId!)?.participantCount ?? 0) > 0 : false;
  const [draft, setDraft] = useState<KecermatanDraft>(() => (testId ? (draftFromTest(testId) ?? blankDraft) : blankDraft));

  const itemTypeOptions: { id: KecermatanItemType; label: string }[] = [
    { id: "char", label: t.itemType.char },
    { id: "number", label: t.itemType.number },
    { id: "symbol", label: t.itemType.symbol },
    { id: "emoji", label: t.itemType.emoji },
    { id: "mix", label: t.itemType.mix },
  ];

  const handleSave = () => {
    if (!draft.title) {
      toast.danger(t.toastIncompleteTitle, { description: t.toastIncompleteDescription });
      return;
    }
    toast.success(isEditing ? t.toastUpdated : t.toastCreated, {
      description: t.toastSummary(draft.title, draft.kolomCount, draft.soalPerKolom),
    });
    router.push("/admin/kecermatan");
  };

  return (
    <AppShell
      title={isEditing ? t.editTitle : t.createTitle}
      description={locked ? t.lockedDescription : undefined}
      actions={
        <Button variant="primary" size="sm" onPress={handleSave} isDisabled={locked}>
          {t.saveTest}
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
            <SelectInput
              label={t.itemTypeLabel}
              options={itemTypeOptions}
              selectedKey={draft.itemType}
              onSelectionChange={(k) => k && setDraft((d) => ({ ...d, itemType: k as KecermatanItemType }))}
              isDisabled={locked}
            />
          </div>
          <TextareaInput
            label={t.descriptionLabel}
            rows={2}
            value={draft.description}
            onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))}
            disabled={locked}
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
          <p className="text-sm font-semibold text-foreground">{t.structureHeading}</p>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label={t.itemLengthLabel}
              type="number"
              min={1}
              max={8}
              value={String(draft.itemLength)}
              onChange={(e) => setDraft((d) => ({ ...d, itemLength: Number(e.target.value) || 1 }))}
              disabled={locked}
            />
            <TextInput
              label={t.optionsPerSoalLabel}
              type="number"
              min={3}
              max={8}
              value={String(draft.optionsPerSoal)}
              onChange={(e) => setDraft((d) => ({ ...d, optionsPerSoal: Number(e.target.value) || 3 }))}
              disabled={locked}
            />
            <TextInput
              label={t.soalPerKolomLabel}
              type="number"
              min={1}
              value={String(draft.soalPerKolom)}
              onChange={(e) => setDraft((d) => ({ ...d, soalPerKolom: Number(e.target.value) || 1 }))}
              disabled={locked}
            />
            <TextInput
              label={t.kolomCountLabel}
              type="number"
              min={1}
              value={String(draft.kolomCount)}
              onChange={(e) => setDraft((d) => ({ ...d, kolomCount: Number(e.target.value) || 1 }))}
              disabled={locked}
            />
            <TextInput
              label={t.kolomDurationLabel}
              type="number"
              min={1}
              value={String(draft.kolomDurationMinutes)}
              onChange={(e) => setDraft((d) => ({ ...d, kolomDurationMinutes: Number(e.target.value) || 1 }))}
              disabled={locked}
            />
          </div>
          <p className="rounded-lg border border-dashed border-border p-3 text-xs text-muted">
            {t.structureNote}
          </p>
        </Card>
      </div>
    </AppShell>
  );
}
