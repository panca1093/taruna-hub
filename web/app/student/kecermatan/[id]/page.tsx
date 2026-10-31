"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@heroui/react";
import { ArrowRight, CloudOff, Gauge, Sparkles } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { Icon } from "@/components/icon";
import { Breadcrumb } from "@/components/page-heading";
import { NoteBox, Panel, SectionHeader } from "@/components/page-blocks";
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
    <AppShell>
      <Breadcrumb items={[{ label: t.crumbDashboard, href: "/student" }, { label: t.crumbKecermatan }]} />
      <div className="grid items-start gap-7 lg:grid-cols-detail">
        <div className="space-y-5.5">
          <Panel tone="soft" radius="hero">
            <div className="mb-4.5 flex flex-wrap items-center gap-3">
              <p className="flex items-center gap-2 text-label text-accent">
                <Icon icon={Sparkles} size="sm" />
                {t.chipKecermatan}
              </p>
              <ProgramBadge code={test.program_code} />
            </div>
            <h1 className="font-display mb-3 max-w-2xl text-display-sm text-balance md:text-display">{test.title}</h1>
            <p className="max-w-prose text-small text-foreground/80">{test.description}</p>
            <div className="mt-6 flex flex-wrap gap-x-7.5 gap-y-4">
              {facts.map((fact) => (
                <div key={fact.label}>
                  <p className="font-display text-metric tabular-nums">{fact.value}</p>
                  <p className="mt-0.5 text-micro text-foreground/80">{fact.label}</p>
                </div>
              ))}
            </div>
          </Panel>

          <Panel>
            <h2 className="font-display mb-3 text-heading-sm">{t.howTitle}</h2>
            <ul className="list-disc space-y-1.5 pl-4.5 text-caption leading-loose text-muted marker:text-rose">
              {steps.map((step) => (
                <li key={step}>{step}</li>
              ))}
            </ul>
            <NoteBox icon={Gauge} className="mt-5">
              {t.quotaNote}
            </NoteBox>
          </Panel>

          <Panel>
            <SectionHeader title={t.historyTitle} />
            {completed.length === 0 ? (
              <p className="text-caption leading-relaxed text-muted">{t.noHistoryInfo}</p>
            ) : (
              <div>
                {completed.map((attempt) => (
                  <Link
                    key={attempt.session_id}
                    href={`/student/kecermatan/${test.test_id}/result?attempt=${attempt.session_id}`}
                    className="flex items-center justify-between gap-6 border-b border-border py-4 text-caption outline-none last:border-0 hover:text-accent focus-visible:ring-2 focus-visible:ring-focus"
                  >
                    <span>
                      {t.attemptLabel(attempt.attempt_number)}
                      {attempt.completed_at && <small className="mt-0.5 block text-micro text-muted">{new Date(attempt.completed_at).toLocaleString()}</small>}
                    </span>
                    <span className="text-right">
                      <span className="font-display text-metric tabular-nums text-foreground">{attempt.nilai_akhir}</span>
                      <small className="block text-micro text-muted">{t.nilaiAkhir}</small>
                    </span>
                  </Link>
                ))}
                <p className="mt-3 text-micro text-muted">{t.historyProvisional}</p>
              </div>
            )}
          </Panel>
        </div>

        <Panel as="aside" className="lg:sticky lg:top-6">
          <p className="mb-3 text-label text-muted">{t.summaryTitle}</p>
          <p className="font-display text-metric-lg tabular-nums">{left}</p>
          <p className="mb-1 text-label text-muted">{t.attemptsLeftUnit}</p>
          <p className="mb-5 text-label text-muted">{t.attemptsUsed(test.attempts_used, test.attempts_purchased)}</p>
          {overview.current ? (
            <>
              <p className="mb-3 text-label leading-relaxed text-muted">{t.inProgressNote}</p>
              <LinkButton href={attemptHref} variant="primary" className="w-full">
                {t.ctaContinue} <Icon icon={ArrowRight} size="sm" />
              </LinkButton>
            </>
          ) : left > 0 ? (
            <LinkButton href={attemptHref} variant="primary" className="w-full">
              {t.ctaStartNew} <Icon icon={ArrowRight} size="sm" />
            </LinkButton>
          ) : (
            <>
              <p className="mb-3 text-label leading-relaxed text-muted">{t.noQuotaNote}</p>
              <LinkButton href={purchaseHref(overview.product_id)} variant="primary" className="w-full">
                {t.ctaBuyMore} <Icon icon={ArrowRight} size="sm" />
              </LinkButton>
            </>
          )}
          {(overview.current || left > 0) && (
            <LinkButton href={purchaseHref(overview.product_id)} variant="secondary" className="mt-3 w-full">
              {t.ctaBuyExtra}
            </LinkButton>
          )}
        </Panel>
      </div>
    </AppShell>
  );
}
