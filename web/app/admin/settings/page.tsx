"use client";

import { useState } from "react";
import { Card } from "@heroui/react";
import { CreditCard, Scale } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { TextInput } from "@/components/ui/form";
import { useSettingsT } from "@/lib/i18n/dict/settings";

export default function SettingsPage() {
  const t = useSettingsT();
  const [weights, setWeights] = useState({ easy: 1, medium: 2, hard: 3 });

  const paymentOptions = [
    { title: t.paymentGatewayTitle, desc: t.paymentGatewayDescription },
    { title: t.manualConfirmTitle, desc: t.manualConfirmDescription },
    { title: t.walletTitle, desc: t.walletDescription },
  ];

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
        </div>
      </AppShell>
    </RequireRole>
  );
}
