"use client";

import { Suspense, use, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@heroui/react";
import { ArrowRight, ChevronDown, CloudOff, Crosshair, Gauge, Mountain, TrendingDown, TrendingUp } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { ProgressLineChart } from "@/components/charts";
import { EmptyState } from "@/components/empty-state";
import { Icon } from "@/components/icon";
import { NoteBox, Panel } from "@/components/page-blocks";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, type KecermatanOverview, type KecermatanResult } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useKecermatanResultT } from "@/lib/i18n/dict/kecermatan-result";

type LoadState = "loading" | "ready" | "failed";

const formatDate = (iso: string) => new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });

function AttemptRecap({ sessionId }: { sessionId: string }) {
  const t = useKecermatanResultT();
  const [result, setResult] = useState<KecermatanResult | null>(null);
  const [failed, setFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.kecermatanRuntime
      .result(sessionId)
      .then((data) => {
        if (!cancelled) setResult(data);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [sessionId, reloadKey]);

  if (failed) {
    return (
      <div className="flex flex-wrap items-center gap-3 pb-5 text-caption text-muted" role="alert">
        {t.attemptLoadError}
        <Button
          size="sm"
          variant="secondary"
          onPress={() => {
            setFailed(false);
            setReloadKey((key) => key + 1);
          }}
        >
          {t.retry}
        </Button>
      </div>
    );
  }
  if (!result) return <p className="pb-5 text-caption text-muted">{t.loadingAttempt}</p>;

  const scores = [
    { label: t.scoreKecepatan, value: result.scores.kecepatan },
    { label: t.scoreKetelitian, value: result.scores.ketelitian },
    { label: t.scoreKetahanan, value: result.scores.ketahanan },
  ];
  const cell = "border-b border-border px-2.5 py-3";

  return (
    <div className="grid items-start gap-7 pb-6 lg:grid-cols-detail">
      <p className="flex flex-wrap gap-x-6 gap-y-1 text-small text-muted tabular-nums lg:col-span-2">
        {scores.map((score) => (
          <span key={score.label}>
            {score.label} <strong className="font-semibold text-foreground">{score.value}</strong>
          </span>
        ))}
      </p>
      <div className="overflow-x-auto">
        <table className="w-full min-w-64 border-collapse text-caption">
          <thead>
            <tr>
              <th className={cn(cell, "text-left text-micro font-medium text-muted")}>{t.kolomColumn}</th>
              <th className={cn(cell, "text-right text-micro font-medium text-muted")}>{t.benar}</th>
              <th className={cn(cell, "text-right text-micro font-medium text-muted")}>{t.salah}</th>
              <th className={cn(cell, "text-right text-micro font-medium text-muted")}>{t.kosong}</th>
            </tr>
          </thead>
          <tbody className="tabular-nums">
            {result.kolom.map((k) => (
              <tr key={k.number}>
                <td className={cn(cell, "text-left")}>{k.number}</td>
                <td className={cn(cell, "text-right")}>{k.benar}</td>
                <td className={cn(cell, "text-right")}>{k.salah}</td>
                <td className={cn(cell, "text-right")}>{k.kosong}</td>
              </tr>
            ))}
          </tbody>
          <tfoot className="font-semibold tabular-nums">
            <tr>
              <td className="px-2.5 py-3 text-left">{t.total}</td>
              <td className="px-2.5 py-3 text-right">{result.benar_total}</td>
              <td className="px-2.5 py-3 text-right">{result.salah_total}</td>
              <td className="px-2.5 py-3 text-right">{result.kosong_total}</td>
            </tr>
          </tfoot>
        </table>
      </div>
      <div>
        <p className="text-label text-muted">{t.correctPerKolomTitle}</p>
        <div className="mt-3 flex h-34 items-end gap-1.5 sm:gap-2.5">
          {result.kolom.map((k) => {
            const total = k.benar + k.salah + k.kosong;
            return (
              <div key={k.number} className="flex h-full min-w-0 flex-1 flex-col items-center justify-end gap-2 text-micro text-muted tabular-nums" title={t.barLabel(k.number, k.benar)}>
                <span>{k.benar}</span>
                <span className="flex min-h-0 w-full flex-1 items-end justify-center">
                  <span className="block min-h-0.5 w-full max-w-9 rounded-t-chip bg-rose" style={{ height: `${total > 0 ? Math.max(2, (k.benar / total) * 100) : 2}%` }} />
                </span>
                <span>{k.number}</span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function ResultContent({ testId }: { testId: string }) {
  const router = useRouter();
  const t = useKecermatanResultT();
  const attemptParam = useSearchParams().get("attempt");
  const [overview, setOverview] = useState<KecermatanOverview | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [openId, setOpenId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.kecermatanRuntime
      .overview(testId)
      .then((data) => {
        if (cancelled) return;
        const completed = data.attempts.filter((attempt) => attempt.status === "completed");
        if (!data.progress || completed.length === 0) {
          router.replace(`/student/kecermatan/${testId}`);
          return;
        }
        setOverview(data);
        setOpenId(completed.find((attempt) => attempt.session_id === attemptParam)?.session_id ?? completed[0].session_id);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          router.replace(`/student/kecermatan/${testId}`);
          return;
        }
        setState("failed");
      });
    return () => {
      cancelled = true;
    };
  }, [testId, attemptParam, router, reloadKey]);

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

  if (state === "loading" || !overview?.progress) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const { test, progress } = overview;
  const completed = overview.attempts.filter((attempt) => attempt.status === "completed");
  const counted = completed.slice(0, progress.attempts_counted);
  const countedIds = new Set(counted.map((attempt) => attempt.session_id));
  const delta = progress.nilai_gabungan_previous === null ? 0 : Math.round((progress.nilai_gabungan - progress.nilai_gabungan_previous) * 100) / 100;
  const averages = [
    { label: t.scoreKecepatan, value: progress.kecepatan, icon: Gauge, tile: "bg-viz-blue-soft", tone: "text-viz-blue" },
    { label: t.scoreKetelitian, value: progress.ketelitian, icon: Crosshair, tile: "bg-viz-green-soft", tone: "text-viz-green" },
    { label: t.scoreKetahanan, value: progress.ketahanan, icon: Mountain, tile: "bg-viz-amber-soft", tone: "text-viz-amber" },
  ];
  const points = [...completed].reverse().map((attempt) => ({
    key: attempt.session_id,
    label: String(attempt.attempt_number),
    value: attempt.nilai_akhir ?? 0,
    counted: countedIds.has(attempt.session_id),
  }));
  const canStart = overview.current !== null || test.attempts_remaining > 0;

  const selectFromChart = (sessionId: string) => {
    setOpenId(sessionId);
    document.getElementById(`attempt-${sessionId}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  };

  return (
    <AppShell
      title={t.heading}
      description={`${test.title} · ${t.attemptsDone(completed.length)}`}
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.crumbResult }]}
      actions={
        canStart ? (
          <LinkButton href={`/student/kecermatan/${testId}/attempt`} variant="primary">
            {t.tryAgainCta} <Icon icon={ArrowRight} size="sm" />
          </LinkButton>
        ) : undefined
      }
    >
      <div className="space-y-6">
        <div className="grid gap-5 sm:gap-7.5 md:grid-cols-score">
          <Panel tone="peach" radius="hero" className="flex flex-col items-center justify-center text-center">
            <p className="text-label text-foreground/80">{t.combinedLabel(progress.attempts_counted)}</p>
            <p className="mt-5 mb-4 font-display text-score tabular-nums">{progress.nilai_gabungan}</p>
            {delta !== 0 && (
              <p className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-label font-medium whitespace-nowrap tabular-nums", delta > 0 ? "bg-viz-green-soft" : "bg-danger-soft")}>
                <Icon icon={delta > 0 ? TrendingUp : TrendingDown} size="sm" className={delta > 0 ? "text-viz-green" : "text-danger"} />
                {delta > 0 ? t.trendUp(delta) : t.trendDown(-delta)}
              </p>
            )}
          </Panel>
          <Panel>
            <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
              <h2 className="font-display text-heading-sm">{t.chartTitle}</h2>
              <p className="flex flex-wrap gap-x-4 gap-y-1 text-micro text-muted">
                <span className="inline-flex items-center gap-1.5">
                  <span className="size-2 rounded-full bg-viz-blue" />
                  {t.legendCounted}
                </span>
                {completed.length > counted.length && (
                  <span className="inline-flex items-center gap-1.5">
                    <span className="size-2 rounded-full border border-rose bg-surface" />
                    {t.legendOlder}
                  </span>
                )}
              </p>
            </div>
            <ProgressLineChart
              className="mt-4"
              points={points}
              selectedKey={openId}
              onSelect={selectFromChart}
              pointLabel={(point) => t.chartPointLabel(Number(point.label), point.value)}
            />
          </Panel>
        </div>

        <div>
          <p className="mb-3 text-small text-muted">{t.averagesCaption(progress.attempts_counted)}</p>
          <div className="grid grid-cols-3 gap-2.5 sm:gap-4">
            {averages.map((item) => (
              <div key={item.label} className={cn("rounded-tile p-4 sm:p-5.5", item.tile)}>
                <Icon icon={item.icon} size="md" className={cn("mb-3", item.tone)} />
                <p className="font-display text-metric tabular-nums sm:text-hero">{item.value}</p>
                <p className="mt-1.5 text-small text-foreground/80">{item.label}</p>
              </div>
            ))}
          </div>
        </div>

        <Panel>
          <div className="mb-1 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <h2 className="font-display text-heading-sm">{t.attemptsTitle}</h2>
            <p className="text-caption text-muted">{t.attemptsHint}</p>
          </div>
          {completed.map((attempt) => {
            const open = attempt.session_id === openId;
            return (
              <div key={attempt.session_id} id={`attempt-${attempt.session_id}`} className="scroll-mt-6 border-b border-border last:border-0">
                <button
                  type="button"
                  aria-expanded={open}
                  aria-controls={`recap-${attempt.session_id}`}
                  onClick={() => setOpenId(open ? null : attempt.session_id)}
                  className="flex w-full items-center gap-4 rounded-chip py-4 text-left text-caption outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  <span className="min-w-0 flex-1">
                    {t.attemptLabel(attempt.attempt_number)}
                    {attempt.completed_at && <small className="mt-0.5 block text-micro text-muted">{formatDate(attempt.completed_at)}</small>}
                  </span>
                  {countedIds.has(attempt.session_id) && (
                    <span className="hidden rounded-full bg-accent-soft px-2.5 py-0.5 text-micro text-accent-soft-foreground sm:inline">{t.countedChip}</span>
                  )}
                  <span className="font-display text-metric tabular-nums">{attempt.nilai_akhir}</span>
                  <Icon icon={ChevronDown} size="sm" className={cn("text-muted transition-transform motion-reduce:transition-none", open && "rotate-180")} />
                </button>
                {open && (
                  <div id={`recap-${attempt.session_id}`}>
                    <AttemptRecap sessionId={attempt.session_id} />
                  </div>
                )}
              </div>
            );
          })}
        </Panel>

        <NoteBox>{t.quotaLeft(test.attempts_remaining)}</NoteBox>

        <div className="flex flex-wrap items-center gap-3.5">
          <LinkButton href={`/student/kecermatan/${testId}`} variant="secondary">
            {t.backToDetail} <Icon icon={ArrowRight} size="xs" />
          </LinkButton>
          <LinkButton href="/student" variant="secondary">
            {t.backToDashboard} <Icon icon={ArrowRight} size="xs" />
          </LinkButton>
        </div>
      </div>
    </AppShell>
  );
}

export default function KecermatanResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ResultContent testId={id} />
    </Suspense>
  );
}
