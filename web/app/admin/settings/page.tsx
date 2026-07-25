"use client";

import { useEffect, useState } from "react";
import { Button, Card, toast } from "@heroui/react";
import { CreditCard, Scale, Settings as SettingsIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { SwitchInput, TextInput } from "@/components/ui/form";
import { getSessionSettings, saveSessionSettings } from "@/lib/session-context";
import { useSettingsT } from "@/lib/i18n/dict/settings";

export default function SettingsPage() {
  const t = useSettingsT();
  const [weights, setWeights] = useState({ easy: 1, medium: 2, hard: 3 });
  const [autoLogout, setAutoLogout] = useState(true);
  const [idleMinutes, setIdleMinutes] = useState("30");

  useEffect(() => {
    const stored = getSessionSettings();
    setAutoLogout(stored.autoLogout);
    setIdleMinutes(String(stored.idleMinutes));
  }, []);

  const handleSave = () => {
    saveSessionSettings({ autoLogout, idleMinutes: Number(idleMinutes) || 30 });
    toast.success(t.settingsSaved);
  };

  const paymentOptions = [
    { title: t.paymentGatewayTitle, desc: t.paymentGatewayDescription },
    { title: t.manualConfirmTitle, desc: t.manualConfirmDescription },
    { title: t.walletTitle, desc: t.walletDescription },
  ];

  return (
    <RequireRole roles={["admin"]} title={t.pageTitle} deniedMessage={t.deniedMessage}>
      <AppShell
        title={t.pageTitle}
        description={t.pageDescription}
        actions={
          <Button variant="primary" size="sm" onPress={handleSave}>
            {t.saveChanges}
          </Button>
        }
      >
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
            <div className="flex items-center gap-2">
              <Scale className="size-4 text-accent" />
              <p className="text-sm font-semibold text-foreground">{t.scoringWeightTitle}</p>
            </div>
            <p className="text-xs text-muted">{t.scoringWeightDescription}</p>
            <div className="grid gap-4 sm:grid-cols-3">
              <TextInput label={t.fieldEasy} type="number" value={String(weights.easy)} onChange={(e) => setWeights((w) => ({ ...w, easy: Number(e.target.value) || 0 }))} />
              <TextInput label={t.fieldMedium} type="number" value={String(weights.medium)} onChange={(e) => setWeights((w) => ({ ...w, medium: Number(e.target.value) || 0 }))} />
              <TextInput label={t.fieldHard} type="number" value={String(weights.hard)} onChange={(e) => setWeights((w) => ({ ...w, hard: Number(e.target.value) || 0 }))} />
            </div>
          </Card>

          <Card className="gap-4 p-5">
            <div className="flex items-center gap-2">
              <SettingsIcon className="size-4 text-accent" />
              <p className="text-sm font-semibold text-foreground">{t.sessionSecurityTitle}</p>
            </div>
            <SwitchInput
              label={t.autoLogoutLabel}
              description={t.autoLogoutDescription}
              isSelected={autoLogout}
              onChange={setAutoLogout}
            />
            {autoLogout && (
              <TextInput
                label={t.idleLimitLabel}
                type="number"
                className="max-w-xs"
                value={idleMinutes}
                onChange={(e) => setIdleMinutes(e.target.value)}
              />
            )}
          </Card>
        </div>
      </AppShell>
    </RequireRole>
  );
}
