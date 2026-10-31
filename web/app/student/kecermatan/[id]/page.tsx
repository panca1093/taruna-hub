"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card } from "@heroui/react";
import { CheckCircle2, CloudOff, Gauge, PlayCircle, ShoppingBag, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { ApiError, api, type KecermatanOverview } from "@/lib/api/client";
import { purchaseHref } from "@/lib/kecermatan-session";
import { useKecermatanOverviewT } from "@/lib/i18n/dict/kecermatan-overview";

type LoadState = "loading" | "ready" | "missing" | "failed";

export default function KecermatanOverviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const t = useKecermatanOverviewT();
  const [overview, setOverview] = useState<KecermatanOverview | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.kecermatanRuntime
      .overview(id)
      .then((data) => {
        if (cancelled) return;
        setOverview(data);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState(err instanceof ApiError && err.status === 404 ? "missing" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [id, reloadKey]);

  if (state === "missing") {
    return (
      <AppShell title={t.pageTitle}>
        <p className="text-sm text-muted">{t.noAccess}</p>
      </AppShell>
    );
  }

  if (state === "failed") {
    return (
      <AppShell title={t.pageTitle}>
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button
              variant="primary"
              size="sm"
              onPress={() => {
                setState("loading");
                setReloadKey((key) => key + 1);
              }}
            >
              {t.retry}
            </Button>
          }
        />
      </AppShell>
    );
  }

  if (state === "loading" || !overview) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      </AppShell>
    );
  }

  const { test, attempts } = overview;
  const left = test.attempts_remaining;
  const completed = attempts.filter((attempt) => attempt.status === "completed");
  const attemptHref = `/student/kecermatan/${test.test_id}/attempt`;
  const lastLabel = String.fromCharCode(64 + test.options_per_soal);
  const facts = [
    { value: test.kolom_count, label: t.factKolom },
    { value: test.soal_per_kolom, label: t.factSoal },
    { value: t.minutes(Math.round(test.kolom_duration_seconds / 60)), label: t.factDuration },
    { value: test.options_per_soal, label: t.factOptions },
  ];
  const steps = [t.howStep1(test.options_per_soal, lastLabel), t.howStep2, t.howStep3, t.howStep4, t.howStep5];

  return (
    <AppShell title={t.pageTitle}>
      <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
        <div className="space-y-6">
          <section className="rounded-2xl border border-border bg-surface p-6">
            <div className="flex items-center gap-2">
              <ProgramBadge code={test.program_code} />
              <span className="rounded-md bg-accent-soft px-2 py-0.5 text-xs font-medium text-accent-soft-foreground">
                {t.chipKecermatan}
              </span>
            </div>
            <h1 className="font-display mt-3 text-2xl font-semibold tracking-tight text-foreground">{test.title}</h1>
            <p className="mt-1.5 text-sm text-muted">{test.description}</p>
            <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
              {facts.map((fact) => (
                <div key={fact.label} className="rounded-xl bg-surface-secondary p-3">
                  <p className="font-display text-xl font-semibold text-foreground">{fact.value}</p>
                  <p className="text-xs text-muted">{fact.label}</p>
                </div>
              ))}
            </div>
          </section>

          <section className="rounded-2xl border border-border bg-surface p-6">
            <h2 className="font-display text-lg font-semibold text-foreground">{t.howTitle}</h2>
            <ul className="mt-3 space-y-2.5 text-sm text-muted">
              {steps.map((step) => (
                <li key={step} className="flex gap-2.5">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-accent" />
                  <span>{step}</span>
                </li>
              ))}
            </ul>
            <p className="mt-4 flex items-start gap-2 rounded-xl bg-surface-secondary p-3 text-xs text-muted">
              <Gauge className="mt-0.5 size-4 shrink-0" />
              {t.quotaNote}
            </p>
          </section>

          <section>
            <h2 className="mb-3 text-sm font-semibold text-foreground">{t.historyTitle}</h2>
            {completed.length === 0 ? (
              <p className="rounded-xl border border-dashed border-border p-4 text-sm text-muted">{t.noHistoryInfo}</p>
            ) : (
              <div className="space-y-2">
                {completed.map((attempt) => (
                  <Link key={attempt.session_id} href={`/student/kecermatan/${test.test_id}/result?attempt=${attempt.session_id}`}>
                    <Card className="flex-row items-center justify-between gap-3 p-3.5 transition-colors hover:bg-surface-secondary">
                      <span className="text-sm font-medium text-foreground">{t.attemptLabel(attempt.attempt_number)}</span>
                      <span className="flex items-center gap-3 text-xs text-muted">
                        {t.nilaiAkhir}{" "}
                        <span className="font-mono font-semibold text-foreground">{attempt.nilai_akhir}</span>
                        {attempt.completed_at && new Date(attempt.completed_at).toLocaleString()}
                      </span>
                    </Card>
                  </Link>
                ))}
                <p className="text-xs text-muted">{t.historyProvisional}</p>
              </div>
            )}
          </section>
        </div>

        <aside className="h-fit space-y-3 rounded-2xl border border-border bg-surface p-5 lg:sticky lg:top-6">
          <p className="text-sm font-semibold text-foreground">{t.summaryTitle}</p>
          <div className="flex items-center gap-2.5">
            <Sparkles className="size-5 text-accent" />
            <div>
              <p className="font-display text-2xl font-semibold text-foreground">{t.attemptsRemaining(left)}</p>
              <p className="text-xs text-muted">{t.attemptsUsed(test.attempts_used, test.attempts_purchased)}</p>
            </div>
          </div>
          {overview.current ? (
            <>
              <p className="text-xs text-muted">{t.inProgressNote}</p>
              <LinkButton href={attemptHref} variant="primary" className="w-full">
                <PlayCircle className="size-4" /> {t.ctaContinue}
              </LinkButton>
            </>
          ) : left > 0 ? (
            <LinkButton href={attemptHref} variant="primary" className="w-full">
              <PlayCircle className="size-4" /> {t.ctaStartNew}
            </LinkButton>
          ) : (
            <>
              <p className="text-xs text-muted">{t.noQuotaNote}</p>
              <LinkButton href={purchaseHref(overview.product_id)} variant="primary" className="w-full">
                <ShoppingBag className="size-4" /> {t.ctaBuyMore}
              </LinkButton>
            </>
          )}
          {(overview.current || left > 0) && (
            <LinkButton href={purchaseHref(overview.product_id)} variant="secondary" className="w-full">
              <ShoppingBag className="size-4" /> {t.ctaBuyExtra}
            </LinkButton>
          )}
        </aside>
      </div>
    </AppShell>
  );
}
