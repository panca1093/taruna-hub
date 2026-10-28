"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { AppShell } from "@/components/app-shell";
import { SelectInput, TextInput, TextareaInput } from "@/components/ui/form";
import { cn } from "@/lib/cn";
import {
  api,
  ApiError,
  errorText,
  type ItemPackage,
  type KecermatanItemType,
  type KecermatanTest,
  type Program,
} from "@/lib/api/client";
import { useAdminKecermatanBuilderT } from "@/lib/i18n/dict/admin-kecermatan-builder";

interface KecermatanDraft {
  title: string;
  programId: string;
  description: string;
  itemType: KecermatanItemType;
  packageId: string;
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

const NO_PACKAGE = "none";
const BUILTIN_POOL = { char: 26, number: 10, shape: 7 } as const;
const MIX_BUILTINS = ["char", "number", "shape"] as const;
const itemTypes: KecermatanItemType[] = ["char", "number", "symbol", "emoji", "image", "shape", "mix"];

const blankDraft: KecermatanDraft = {
  title: "",
  programId: "",
  description: "",
  itemType: "char",
  packageId: NO_PACKAGE,
  itemLength: 4,
  optionsPerSoal: 5,
  soalPerKolom: 50,
  kolomCount: 10,
  kolomDurationMinutes: 2,
  coverFrom: covers[0][0],
  coverTo: covers[0][1],
};

function draftOf(test: KecermatanTest): KecermatanDraft {
  return {
    title: test.title,
    programId: test.program_id,
    description: test.description,
    itemType: test.item_type,
    packageId: test.packages[0]?.package_id ?? NO_PACKAGE,
    itemLength: test.item_length,
    optionsPerSoal: test.options_per_soal,
    soalPerKolom: test.soal_per_kolom,
    kolomCount: test.kolom_count,
    kolomDurationMinutes: Math.max(1, Math.round(test.kolom_duration_seconds / 60)),
    coverFrom: test.cover_from,
    coverTo: test.cover_to,
  };
}

const packageRequired = (type: KecermatanItemType) => type === "symbol" || type === "emoji" || type === "image";
const packageAllowed = (type: KecermatanItemType) => packageRequired(type) || type === "shape" || type === "mix";

export function KecermatanBuilder({ testId }: { testId?: string }) {
  const router = useRouter();
  const t = useAdminKecermatanBuilderT();
  const isEditing = !!testId;

  const [programs, setPrograms] = useState<Program[] | null>(null);
  const [packages, setPackages] = useState<ItemPackage[] | null>(null);
  const [test, setTest] = useState<KecermatanTest | null>(null);
  const [draft, setDraft] = useState<KecermatanDraft>(blankDraft);
  const [loading, setLoading] = useState(isEditing);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [saving, setSaving] = useState(false);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});

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
    api.itemPackages
      .list()
      .then(({ packages }) => {
        if (!cancelled) setPackages(packages);
      })
      .catch(() => {
        if (!cancelled) setPackages([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!testId) return;
    let cancelled = false;
    api.kecermatan
      .get(testId)
      .then((loaded) => {
        if (cancelled) return;
        setTest(loaded);
        setDraft(draftOf(loaded));
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
  }, [testId, reloadKey]);

  const retry = () => {
    setLoading(true);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const locked = test?.locked ?? false;
  const selectedProgramId = draft.programId !== "" ? draft.programId : (programs?.[0]?.id ?? "");
  const programOptions = (programs ?? []).map((p) => ({ id: p.id, label: p.name }));
  const itemTypeOptions = itemTypes.map((type) => ({ id: type, label: t.itemType[type] }));

  const candidates = (packages ?? []).filter((p) => (draft.itemType === "mix" ? true : p.type === draft.itemType));
  const chosen = candidates.find((p) => p.id === draft.packageId);
  const packageOptions = [
    ...(packageRequired(draft.itemType) ? [] : [{ id: NO_PACKAGE, label: draft.itemType === "shape" ? t.packageNoneShape : t.packageNone }]),
    ...candidates.map((p) => ({ id: p.id, label: t.packageOption(p.name, p.element_count) })),
  ];

  const need = draft.itemLength * draft.optionsPerSoal;
  const builtinPool =
    draft.itemType === "char" || draft.itemType === "number"
      ? BUILTIN_POOL[draft.itemType]
      : draft.itemType === "shape"
        ? chosen
          ? 0
          : BUILTIN_POOL.shape
        : draft.itemType === "mix"
          ? BUILTIN_POOL.char + BUILTIN_POOL.number + BUILTIN_POOL.shape
          : 0;
  const pool = builtinPool + (chosen?.element_count ?? 0);

  const bodyOf = () => ({
    program_id: selectedProgramId,
    title: draft.title,
    description: draft.description,
    item_type: draft.itemType,
    item_length: draft.itemLength,
    options_per_soal: draft.optionsPerSoal,
    soal_per_kolom: draft.soalPerKolom,
    kolom_count: draft.kolomCount,
    kolom_duration_seconds: Math.round(draft.kolomDurationMinutes * 60),
    builtin_sources: draft.itemType === "mix" ? [...MIX_BUILTINS] : [],
    package_ids: packageAllowed(draft.itemType) && chosen ? [chosen.id] : [],
    cover_from: draft.coverFrom,
    cover_to: draft.coverTo,
  });

  const handleSave = async () => {
    if (!draft.title.trim()) {
      toast.danger(t.toastIncompleteTitle, { description: t.toastIncompleteDescription });
      return;
    }
    setSaving(true);
    setFormErrors({});
    try {
      if (test) {
        const saved = await api.kecermatan.update(test.id, { ...bodyOf(), base_revision: test.revision });
        setTest(saved);
        setDraft(draftOf(saved));
        toast.success(t.toastUpdated, { description: t.toastSummary(saved.title, saved.kolom_count, saved.soal_per_kolom) });
      } else {
        const created = await api.kecermatan.create(bodyOf());
        toast.success(t.toastCreated, { description: t.toastSummary(created.title, created.kolom_count, created.soal_per_kolom) });
        router.push("/admin/kecermatan");
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

  if (loading) {
    return (
      <AppShell title={t.editTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      </AppShell>
    );
  }

  if (loadFailed) {
    return (
      <AppShell title={t.editTitle}>
        <div className="py-14 text-center">
          <p className="text-sm font-medium text-foreground">{t.loadErrorTitle}</p>
          <p className="mt-1 text-sm text-muted">{t.loadErrorDescription}</p>
          <Button variant="primary" size="sm" className="mt-4" onPress={retry}>
            {t.retry}
          </Button>
        </div>
      </AppShell>
    );
  }

  const numberField = (key: "itemLength" | "optionsPerSoal" | "soalPerKolom" | "kolomCount" | "kolomDurationMinutes", fallback: number) => ({
    type: "number" as const,
    value: String(draft[key]),
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => setDraft((d) => ({ ...d, [key]: Number(e.target.value) || fallback })),
    disabled: locked,
  });

  return (
    <AppShell
      title={isEditing ? t.editTitle : t.createTitle}
      description={locked ? t.lockedDescription : undefined}
      actions={
        <Button variant="primary" size="sm" onPress={handleSave} isDisabled={locked || saving}>
          {saving ? t.saving : t.saveTest}
        </Button>
      }
    >
      <div className="mx-auto max-w-3xl space-y-6">
        {test && <p className="text-xs text-muted">{t.revisionLabel(test.revision)}</p>}

        <Card className="gap-4 p-5">
          <p className="text-sm font-semibold text-foreground">{t.infoHeading}</p>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label={t.titleLabel}
              required
              className="sm:col-span-2"
              value={draft.title}
              error={formErrors.title}
              onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
              disabled={locked}
            />
            <SelectInput
              label={t.programLabel}
              options={programOptions}
              selectedKey={selectedProgramId}
              error={formErrors.program_id}
              onSelectionChange={(k) => k && setDraft((d) => ({ ...d, programId: k }))}
              isDisabled={locked}
            />
            <SelectInput
              label={t.itemTypeLabel}
              options={itemTypeOptions}
              selectedKey={draft.itemType}
              error={formErrors.item_type}
              onSelectionChange={(k) => k && setDraft((d) => ({ ...d, itemType: k as KecermatanItemType, packageId: NO_PACKAGE }))}
              isDisabled={locked}
            />
            {packageAllowed(draft.itemType) && (
              <SelectInput
                className="sm:col-span-2"
                label={packageRequired(draft.itemType) ? t.packageLabel : t.packageOptionalLabel}
                options={packageOptions}
                selectedKey={draft.packageId}
                error={formErrors["package_ids[0]"] ?? formErrors.package_ids}
                description={
                  packages !== null && candidates.length === 0 && packageRequired(draft.itemType)
                    ? t.packageMissing(t.itemType[draft.itemType])
                    : undefined
                }
                onSelectionChange={(k) => k && setDraft((d) => ({ ...d, packageId: k }))}
                isDisabled={locked}
              />
            )}
          </div>
          <TextareaInput
            label={t.descriptionLabel}
            rows={2}
            value={draft.description}
            error={formErrors.description}
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
            <TextInput label={t.itemLengthLabel} min={1} max={5} error={formErrors.item_length} {...numberField("itemLength", 1)} />
            <TextInput label={t.optionsPerSoalLabel} min={3} max={8} error={formErrors.options_per_soal} {...numberField("optionsPerSoal", 3)} />
            <TextInput label={t.soalPerKolomLabel} min={50} max={100} error={formErrors.soal_per_kolom} {...numberField("soalPerKolom", 50)} />
            <TextInput label={t.kolomCountLabel} min={10} max={50} step={2} error={formErrors.kolom_count} {...numberField("kolomCount", 10)} />
            <TextInput label={t.kolomDurationLabel} min={1} max={60} error={formErrors.kolom_duration_seconds} {...numberField("kolomDurationMinutes", 1)} />
          </div>
          <p className={cn("text-xs", pool < need || formErrors.pool ? "text-danger" : "text-muted")}>
            {t.poolSummary(pool, need)}
            {(pool < need || formErrors.pool) && ` — ${t.poolShort}`}
          </p>
          {draft.itemType === "mix" && <p className="text-xs text-muted">{t.mixNote}</p>}
          <p className="rounded-lg border border-dashed border-border p-3 text-xs text-muted">
            {t.structureNote}
          </p>
        </Card>
      </div>
    </AppShell>
  );
}
