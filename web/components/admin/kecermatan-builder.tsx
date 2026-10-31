"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { Info } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { NoteBox, Panel } from "@/components/page-blocks";
import { SummaryBox } from "@/components/admin/summary-box";
import { KecermatanPreviewModal } from "@/components/admin/kecermatan-preview-modal";
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
  builtinSources: KecermatanTest["builtin_sources"];
  packageIds: string[];
  itemLength: string;
  optionsPerSoal: string;
  soalPerKolom: string;
  kolomCount: string;
  kolomDurationMinutes: string;
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
  builtinSources: [],
  packageIds: [],
  itemLength: "4",
  optionsPerSoal: "5",
  soalPerKolom: "50",
  kolomCount: "10",
  kolomDurationMinutes: "2",
  coverFrom: covers[0][0],
  coverTo: covers[0][1],
};

const durationMinutesOf = (test: KecermatanTest) => Math.max(1, Math.round(test.kolom_duration_seconds / 60));

const numberFields = {
  itemLength: { min: 1, max: 5, path: "item_length" },
  optionsPerSoal: { min: 3, max: 8, path: "options_per_soal" },
  soalPerKolom: { min: 50, max: 100, path: "soal_per_kolom" },
  kolomCount: { min: 10, max: 50, path: "kolom_count" },
  kolomDurationMinutes: { min: 1, max: 60, path: "kolom_duration_seconds" },
} as const;
type NumberKey = keyof typeof numberFields;

function draftOf(test: KecermatanTest): KecermatanDraft {
  return {
    title: test.title,
    programId: test.program_id,
    description: test.description,
    itemType: test.item_type,
    builtinSources: test.item_type === "mix" ? [...test.builtin_sources] : [],
    packageIds: test.packages.map((p) => p.package_id),
    itemLength: String(test.item_length),
    optionsPerSoal: String(test.options_per_soal),
    soalPerKolom: String(test.soal_per_kolom),
    kolomCount: String(test.kolom_count),
    kolomDurationMinutes: String(durationMinutesOf(test)),
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
  const [touched, setTouched] = useState<Set<NumberKey>>(new Set());
  const [previewOpen, setPreviewOpen] = useState(false);

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
  const firstPackageId = draft.packageIds[0] ?? NO_PACKAGE;
  const keptPackages = Math.max(0, draft.packageIds.length - 1);
  const packageOptions = [
    ...(packageRequired(draft.itemType) ? [] : [{ id: NO_PACKAGE, label: draft.itemType === "shape" ? t.packageNoneShape : t.packageNone }]),
    ...candidates.map((p) => ({ id: p.id, label: t.packageOption(p.name, p.element_count) })),
    // A pinned package the list no longer offers for this type stays selectable, so it is never silently dropped.
    ...(firstPackageId !== NO_PACKAGE && !candidates.some((p) => p.id === firstPackageId)
      ? [{ id: firstPackageId, label: test?.packages.find((p) => p.package_id === firstPackageId)?.name ?? firstPackageId }]
      : []),
  ];

  const numeric = (key: NumberKey) => (/^\d+$/.test(draft[key].trim()) ? Number(draft[key]) : null);
  const numberError = (key: NumberKey): string | undefined => {
    const { min, max } = numberFields[key];
    const value = numeric(key);
    if (value === null || value < min || value > max) return t.rangeError(min, max);
    if (key === "kolomCount" && value % 2 !== 0) return t.evenError;
    return undefined;
  };
  const fieldError = (key: NumberKey) => (touched.has(key) ? numberError(key) : undefined) ?? formErrors[numberFields[key].path];

  const need = (numeric("itemLength") ?? 0) * (numeric("optionsPerSoal") ?? 0);
  const packageCount = (pid: string) =>
    packages?.find((p) => p.id === pid)?.element_count ?? test?.packages.find((p) => p.package_id === pid)?.element_count ?? 0;
  const builtinPool =
    draft.itemType === "char" || draft.itemType === "number"
      ? BUILTIN_POOL[draft.itemType]
      : draft.itemType === "shape"
        ? draft.packageIds.length > 0
          ? 0
          : BUILTIN_POOL.shape
        : draft.itemType === "mix"
          ? draft.builtinSources.reduce((sum, name) => sum + (BUILTIN_POOL[name as keyof typeof BUILTIN_POOL] ?? 0), 0)
          : 0;
  const pool = builtinPool + draft.packageIds.reduce((sum, pid) => sum + packageCount(pid), 0);

  const selectType = (type: KecermatanItemType) =>
    setDraft((d) => ({ ...d, itemType: type, packageIds: [], builtinSources: type === "mix" ? [...MIX_BUILTINS] : [] }));
  const selectPackage = (key: string) =>
    setDraft((d) => ({ ...d, packageIds: key === NO_PACKAGE ? d.packageIds.slice(1) : [key, ...d.packageIds.slice(1)] }));

  const durationSeconds = () =>
    test && draft.kolomDurationMinutes === String(durationMinutesOf(test))
      ? test.kolom_duration_seconds
      : (numeric("kolomDurationMinutes") ?? 0) * 60;

  const bodyOf = () => ({
    program_id: selectedProgramId,
    title: draft.title,
    description: draft.description,
    item_type: draft.itemType,
    item_length: numeric("itemLength") ?? 0,
    options_per_soal: numeric("optionsPerSoal") ?? 0,
    soal_per_kolom: numeric("soalPerKolom") ?? 0,
    kolom_count: numeric("kolomCount") ?? 0,
    kolom_duration_seconds: durationSeconds(),
    builtin_sources: draft.itemType === "mix" ? draft.builtinSources : [],
    package_ids: packageAllowed(draft.itemType) ? draft.packageIds : [],
    cover_from: draft.coverFrom,
    cover_to: draft.coverTo,
  });

  const handleSave = async () => {
    if (!draft.title.trim()) {
      toast.danger(t.toastIncompleteTitle, { description: t.toastIncompleteDescription });
      return;
    }
    setTouched(new Set(Object.keys(numberFields) as NumberKey[]));
    if ((Object.keys(numberFields) as NumberKey[]).some((key) => numberError(key))) return;
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

  const numberField = (key: NumberKey) => ({
    type: "number" as const,
    value: draft[key],
    error: fieldError(key),
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => setDraft((d) => ({ ...d, [key]: e.target.value })),
    onBlur: () => setTouched((prev) => new Set(prev).add(key)),
    disabled: locked,
  });
  const dirty = test !== null && JSON.stringify(draft) !== JSON.stringify(draftOf(test));

  return (
    <AppShell
      title={isEditing ? t.editTitle : t.createTitle}
      description={locked ? t.lockedDescription : undefined}
      breadcrumb={[{ label: t.listCrumb, href: "/admin/kecermatan" }, { label: isEditing ? t.editTitle : t.createTitle }]}
      actions={
        <div className="flex gap-2">
          {test && (
            <Button variant="secondary" size="sm" isDisabled={dirty || saving} onPress={() => setPreviewOpen(true)}>
              {t.previewButton}
            </Button>
          )}
          <Button variant="primary" size="sm" onPress={handleSave} isDisabled={locked || saving}>
            {saving ? t.saving : t.saveTest}
          </Button>
        </div>
      }
    >
      <div className="grid items-start gap-6 lg:grid-cols-3">
        <div className="min-w-0 space-y-6 lg:col-span-2">
        {test && (
          <p className="text-caption text-muted">
            {t.revisionLabel(test.revision)}
            {dirty && ` · ${t.saveBeforePreview}`}
          </p>
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
              onSelectionChange={(k) => k && selectType(k as KecermatanItemType)}
              isDisabled={locked}
            />
            {packageAllowed(draft.itemType) && (
              <SelectInput
                className="sm:col-span-2"
                label={packageRequired(draft.itemType) ? t.packageLabel : t.packageOptionalLabel}
                options={packageOptions}
                selectedKey={firstPackageId}
                error={formErrors["package_ids[0]"] ?? formErrors.package_ids}
                description={
                  keptPackages > 0
                    ? t.packagesKept(keptPackages)
                    : packages !== null && candidates.length === 0 && packageRequired(draft.itemType)
                      ? t.packageMissing(t.itemType[draft.itemType])
                      : undefined
                }
                onSelectionChange={(k) => k && selectPackage(k)}
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
            <p className="mb-2 text-small font-medium text-foreground">{t.coverColorLabel}</p>
            <div className="flex gap-2">
              {covers.map(([from, to]) => (
                <button
                  key={from + to}
                  type="button"
                  disabled={locked}
                  onClick={() => setDraft((d) => ({ ...d, coverFrom: from, coverTo: to }))}
                  className={cn(
                    "size-8 rounded-control ring-offset-2 ring-offset-surface transition-shadow disabled:cursor-not-allowed disabled:opacity-40",
                    draft.coverFrom === from && draft.coverTo === to && "ring-2 ring-accent",
                  )}
                  style={{ background: `linear-gradient(135deg, ${from}, ${to})` }}
                  aria-label={t.coverColorAria}
                />
              ))}
            </div>
          </div>
        </Panel>

        <Panel as="section" className="flex flex-col gap-4">
          <h2 className="font-display text-heading-sm">{t.structureHeading}</h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput label={t.itemLengthLabel} min={1} max={5} {...numberField("itemLength")} />
            <TextInput label={t.optionsPerSoalLabel} min={3} max={8} {...numberField("optionsPerSoal")} />
            <TextInput label={t.soalPerKolomLabel} min={50} max={100} {...numberField("soalPerKolom")} />
            <TextInput label={t.kolomCountLabel} min={10} max={50} step={2} {...numberField("kolomCount")} />
            <TextInput label={t.kolomDurationLabel} min={1} max={60} {...numberField("kolomDurationMinutes")} />
          </div>
          <p className={cn("text-caption", pool < need || formErrors.pool ? "text-danger" : "text-muted")}>
            {t.poolSummary(pool, need)}
            {(pool < need || formErrors.pool) && ` — ${t.poolShort}`}
          </p>
          {draft.itemType === "mix" && <p className="text-caption text-muted">{t.mixNote}</p>}
          <NoteBox icon={Info}>{t.structureNote}</NoteBox>
        </Panel>
        </div>

        <SummaryBox
          title={t.summaryHeading}
          lines={[
            { label: t.summaryColumns, value: numeric("kolomCount") ?? "–" },
            { label: t.summaryQuestionsPerColumn, value: numeric("soalPerKolom") ?? "–" },
            { label: t.summaryTotal, value: (numeric("kolomCount") ?? 0) * (numeric("soalPerKolom") ?? 0) },
            { label: t.summaryDuration, value: t.summaryDurationValue(numeric("kolomDurationMinutes") ?? 0) },
            { label: t.summaryPool, value: `${pool} / ${need}` },
          ]}
        />
      </div>
      {test && <KecermatanPreviewModal testId={test.id} kolomCount={test.kolom_count} isOpen={previewOpen} onOpenChange={setPreviewOpen} />}
    </AppShell>
  );
}
