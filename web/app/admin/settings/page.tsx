"use client";

import { useEffect, useState } from "react";
import { Button, Card, toast } from "@heroui/react";
import { Banknote, CreditCard, Scale } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { TextInput } from "@/components/ui/form";
import { api, ApiError, errorText } from "@/lib/api/client";
import { useSettingsT } from "@/lib/i18n/dict/settings";

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
        <div className="mx-auto max-w-2xl space-y-6">
          <Card className="gap-4 p-5">
            <div className="flex items-center gap-2">
              <CreditCard className="size-4 text-accent" />
              <p className="text-sm font-semibold text-foreground">{t.paymentModelTitle}</p>
            </div>
            <p className="text-xs text-muted">{t.paymentModelDescription}</p>
            <div className="grid gap-3 sm:grid-cols-3">
              {paymentOptions.map((opt) => (
                <div key={opt.title} className="rounded-lg border border-border p-3.5">
                  <p className="text-sm font-medium text-foreground">{opt.title}</p>
                  <p className="mt-1 text-xs text-muted">{opt.desc}</p>
                </div>
              ))}
            </div>
          </Card>

          <Card className="gap-4 p-5">
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Scale className="size-4 text-accent" />
                <p className="text-sm font-semibold text-foreground">{t.scoringWeightTitle}</p>
              </div>
              <Button variant="primary" size="sm" onPress={save} isDisabled={!dirty || saving}>
                {saving ? t.saving : t.save}
              </Button>
            </div>
            <p className="text-xs text-muted">{t.scoringWeightDescription}</p>
            {loadFailed ? (
              <div className="py-4 text-center">
                <p className="text-sm text-muted">{t.loadErrorDescription}</p>
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
              <p className="py-4 text-center text-sm text-muted">{t.loading}</p>
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
          </Card>

          <Card className="gap-4 p-5">
            <div className="flex items-center gap-2">
              <Banknote className="size-4 text-accent" />
              <p className="text-sm font-semibold text-foreground">{t.minTopupTitle}</p>
            </div>
            <p className="text-xs text-muted">{t.minTopupDescription}</p>
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
          </Card>
        </div>
      </AppShell>
    </RequireRole>
  );
}
