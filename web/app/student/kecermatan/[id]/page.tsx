"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card } from "@heroui/react";
import { CloudOff, Gauge, PlayCircle, ShoppingBag, Sparkles } from "lucide-react";
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

  return (
    <AppShell title={t.pageTitle}>
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center gap-2">
          <ProgramBadge code={test.program_code} />
          <span className="rounded-md bg-accent-soft px-2 py-0.5 text-xs font-medium text-accent-soft-foreground">
            {t.chipKecermatan}
          </span>
        </div>
        <h1 className="font-display mt-2 text-2xl font-semibold tracking-tight text-foreground">{test.title}</h1>
        <p className="mt-1.5 text-sm text-muted">{test.description}</p>

        <Card className="mt-6 flex-row items-center justify-between gap-3 p-4">
          <div className="flex items-center gap-2.5">
            <Sparkles className="size-5 text-accent" />
            <div>
              <p className="text-sm font-medium text-foreground">{t.attemptsRemaining(left)}</p>
              <p className="text-xs text-muted">
                {t.kolomStat(test.kolom_count, test.soal_per_kolom, Math.round(test.kolom_duration_seconds / 60))}
              </p>
            </div>
          </div>
          {overview.current ? (
            <LinkButton href={`/student/kecermatan/${test.test_id}/attempt`} variant="primary" size="sm">
              <PlayCircle className="size-3.5" /> {t.ctaContinue}
            </LinkButton>
          ) : left > 0 ? (
            <LinkButton href={`/student/kecermatan/${test.test_id}/attempt`} variant="primary" size="sm">
              <PlayCircle className="size-3.5" /> {t.ctaStartNew}
            </LinkButton>
          ) : (
            <LinkButton href={purchaseHref(overview.product_id)} variant="secondary" size="sm">
              <ShoppingBag className="size-3.5" /> {t.ctaBuyMore}
            </LinkButton>
          )}
        </Card>

        {completed.length > 0 && (
          <div className="mt-6">
            <p className="mb-2 text-sm font-semibold text-foreground">{t.historyTitle}</p>
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
            </div>
          </div>
        )}

        {completed.length === 0 && (
          <div className="mt-6 flex items-start gap-3 rounded-xl border border-dashed border-border p-4 text-sm text-muted">
            <Gauge className="mt-0.5 size-4 shrink-0" />
            <p>{t.noHistoryInfo}</p>
          </div>
        )}
      </div>
    </AppShell>
  );
}
