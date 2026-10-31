"use client";

import { useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { Banknote, CreditCard, Scale, type LucideIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { Icon } from "@/components/icon";
import { LoadingNote } from "@/components/admin-detail";
import { Panel } from "@/components/page-blocks";
import { RequireRole } from "@/components/require-role";
import { TextInput } from "@/components/ui/form";
import { api, ApiError, errorText } from "@/lib/api/client";
import { useSettingsT } from "@/lib/i18n/dict/settings";

function SectionTitle({ icon, children }: { icon: LucideIcon; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span className="grid size-9.5 shrink-0 place-items-center rounded-control bg-peach text-accent">
        <Icon icon={icon} size="sm" />
      </span>
      <p className="font-display text-heading-sm text-foreground">{children}</p>
    </div>
  );
}

type Weights = { easy: string; medium: string; hard: string; minTopup: string };

const wholeRupiah = (value: string) => value.split(".")[0];

export default function SettingsPage() {
  const t = useSettingsT();
  const [weights, setWeights] = useState<Weights | null>(null);
  const [saved, setSaved] = useState<Weights | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    let cancelled = false;
    api.settings
      .get()
      .then(({ scoring_weights: w, min_topup }) => {
        if (cancelled) return;
        const loaded = { easy: String(w.easy), medium: String(w.medium), hard: String(w.hard), minTopup: wholeRupiah(min_topup) };
        setWeights(loaded);
        setSaved(loaded);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const save = async () => {
    if (!weights) return;
    setSaving(true);
    setErrors({});
    try {
      const result = await api.settings.update({
        scoring_weights: { easy: Number(weights.easy), medium: Number(weights.medium), hard: Number(weights.hard) },
        min_topup: weights.minTopup,
      });
      const next = {
        easy: String(result.scoring_weights.easy),
        medium: String(result.scoring_weights.medium),
        hard: String(result.scoring_weights.hard),
        minTopup: wholeRupiah(result.min_topup),
      };
      setWeights(next);
      setSaved(next);
      toast.success(t.toastSaved);
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields);
      toast.danger(t.toastSaveFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  const dirty = weights !== null && saved !== null && JSON.stringify(weights) !== JSON.stringify(saved);

  const paymentOptions = [
    { title: t.paymentGatewayTitle, desc: t.paymentGatewayDescription },
    { title: t.manualConfirmTitle, desc: t.manualConfirmDescription },
    { title: t.walletTitle, desc: t.walletDescription },
  ];

  const minTopupError = errors["min_topup"];

  return (
    <RequireRole roles={["admin"]} title={t.pageTitle} deniedMessage={t.deniedMessage}>
      <AppShell title={t.pageTitle} description={t.pageDescription}>
        <div className="mx-auto max-w-3xl space-y-6">
          <Panel className="flex flex-col gap-4">
            <SectionTitle icon={CreditCard}>{t.paymentModelTitle}</SectionTitle>
            <p className="text-caption text-muted">{t.paymentModelDescription}</p>
            <div className="grid gap-3 sm:grid-cols-3">
              {paymentOptions.map((opt) => (
                <div key={opt.title} className="rounded-tile border border-border bg-background p-4">
                  <p className="text-small font-medium text-foreground">{opt.title}</p>
                  <p className="mt-1 text-label text-muted">{opt.desc}</p>
                </div>
              ))}
            </div>
          </Panel>

          <Panel className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-2">
              <SectionTitle icon={Scale}>{t.scoringWeightTitle}</SectionTitle>
              <Button variant="primary" size="sm" onPress={save} isDisabled={!dirty || saving}>
                {saving ? t.saving : t.save}
              </Button>
            </div>
            <p className="text-caption text-muted">{t.scoringWeightDescription}</p>
            {loadFailed ? (
              <div className="py-4 text-center">
                <p className="text-small text-muted">{t.loadErrorDescription}</p>
                <Button
                  variant="primary"
                  size="sm"
                  className="mt-3"
                  onPress={() => {
                    setLoadFailed(false);
                    setReloadKey((key) => key + 1);
                  }}
                >
                  {t.retry}
                </Button>
              </div>
            ) : weights === null ? (
              <LoadingNote>{t.loading}</LoadingNote>
            ) : (
              <div className="grid gap-4 sm:grid-cols-3">
                <TextInput
                  label={t.fieldEasy}
                  type="number"
                  min={0.01}
                  step={0.5}
                  value={weights.easy}
                  error={errors["weights.easy"]}
                  onChange={(e) => setWeights((w) => (w ? { ...w, easy: e.target.value } : w))}
                />
                <TextInput
                  label={t.fieldMedium}
                  type="number"
                  min={0.01}
                  step={0.5}
                  value={weights.medium}
                  error={errors["weights.medium"]}
                  onChange={(e) => setWeights((w) => (w ? { ...w, medium: e.target.value } : w))}
                />
                <TextInput
                  label={t.fieldHard}
                  type="number"
                  min={0.01}
                  step={0.5}
                  value={weights.hard}
                  error={errors["weights.hard"]}
                  onChange={(e) => setWeights((w) => (w ? { ...w, hard: e.target.value } : w))}
                />
              </div>
            )}
          </Panel>

          <Panel className="flex flex-col gap-4">
            <SectionTitle icon={Banknote}>{t.minTopupTitle}</SectionTitle>
            <p className="text-caption text-muted">{t.minTopupDescription}</p>
            {weights !== null && (
              <div className="max-w-xs">
                <TextInput
                  label={t.fieldMinTopup}
                  type="number"
                  inputMode="numeric"
                  min={1}
                  step={1000}
                  value={weights.minTopup}
                  error={minTopupError}
                  onChange={(e) => setWeights((w) => (w ? { ...w, minTopup: e.target.value } : w))}
                />
              </div>
            )}
          </Panel>
        </div>
      </AppShell>
    </RequireRole>
  );
}
