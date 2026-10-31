"use client";

import { Suspense, use, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { CheckCircle2, CloudOff, Gauge, Timer } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { EmptyState } from "@/components/empty-state";
import { KecermatanItem } from "@/components/kecermatan-item";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, type KecermatanOverview, type KecermatanReview } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useKecermatanResultT } from "@/lib/i18n/dict/kecermatan-result";

type LoadState = "loading" | "ready" | "failed";

const PAGE_SIZE = 10;

function ResultContent({ testId }: { testId: string }) {
  const router = useRouter();
  const t = useKecermatanResultT();
  const attemptParam = useSearchParams().get("attempt");
  const [review, setReview] = useState<KecermatanReview | null>(null);
  const [overview, setOverview] = useState<KecermatanOverview | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [kolomPick, setKolomPick] = useState(1);
  const [page, setPage] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const overviewRequest = api.kecermatanRuntime.overview(testId).catch(() => null);
      let sessionId = attemptParam;
      if (!sessionId) {
        const current = await overviewRequest;
        sessionId = current?.attempts.find((attempt) => attempt.status === "completed")?.session_id ?? null;
      }
      if (!sessionId) throw new ApiError(404, { code: "not_found", message: "" });
      const [result, detail, list] = await Promise.all([
        api.kecermatanRuntime.result(sessionId),
        api.kecermatanRuntime.review(sessionId),
        overviewRequest,
      ]);
      return { review: { ...detail, result: { ...detail.result, kolom: result.kolom } }, list };
    };
    load()
      .then((data) => {
        if (cancelled) return;
        setReview(data.review);
        setOverview(data.list);
        setKolomPick(1);
        setPage(0);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && (err.status === 404 || err.code === "kecermatan_result_not_ready")) {
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

  if (state === "loading" || !review) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const { result } = review;
  const counts = [
    { label: t.benar, value: result.benar_total, tone: "text-success" },
    { label: t.salah, value: result.salah_total, tone: "text-danger" },
    { label: t.kosong, value: result.kosong_total, tone: "text-muted" },
  ];
  const scoreCards = [
    { label: t.scoreKecepatan, value: result.scores.kecepatan, icon: Timer },
    { label: t.scoreKetahanan, value: result.scores.ketahanan, icon: Gauge },
    { label: t.scoreKetelitian, value: result.scores.ketelitian, icon: CheckCircle2 },
  ];
  const completedAttempts = overview?.attempts.filter((attempt) => attempt.status === "completed") ?? [];

  const picked = review.kolom.find((item) => item.number === kolomPick) ?? review.kolom[0];
  const column = picked ? review.columns[picked.source_column - 1] : undefined;
  const soalTotal = column?.soal.length ?? 0;
  const pageCount = Math.max(1, Math.ceil(soalTotal / PAGE_SIZE));
  const from = page * PAGE_SIZE;
  const to = Math.min(soalTotal, from + PAGE_SIZE);
  const statusLabel = { benar: t.statusBenar, salah: t.statusSalah, kosong: t.statusKosong };
  const statusTone = {
    benar: "bg-success-soft text-success-soft-foreground",
    salah: "bg-danger-soft text-danger-soft-foreground",
    kosong: "bg-default text-default-foreground",
  };

  return (
    <AppShell title={t.heading} description={result.test_title}>
      <div className="space-y-6">
        <div className="grid gap-4 lg:grid-cols-[18rem_1fr]">
          <Card className="items-center justify-center gap-3 p-6 text-center">
            <CountdownRing value={result.scores.nilai_akhir} size={120} strokeWidth={9} animate>
              <div className="text-center">
                <p className="font-display text-3xl font-semibold text-foreground">{result.scores.nilai_akhir}</p>
                <p className="text-xs text-muted">{t.outOf100}</p>
              </div>
            </CountdownRing>
            <p className="text-sm font-medium text-muted">{t.nilaiAkhirLabel(result.attempt_number)}</p>
          </Card>
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-3">
              {counts.map((item) => (
                <Card key={item.label} className="gap-1 p-4">
                  <p className={cn("font-display text-2xl font-semibold", item.tone)}>{item.value}</p>
                  <p className="text-xs text-muted">{item.label}</p>
                </Card>
              ))}
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              {scoreCards.map((s) => (
                <Card key={s.label} className="gap-2 p-4">
                  <span className="inline-flex size-8 items-center justify-center rounded-lg bg-accent-soft text-accent-soft-foreground">
                    <s.icon className="size-4" />
                  </span>
                  <p className="font-display text-xl font-semibold text-foreground">{s.value}</p>
                  <p className="text-xs text-muted">{s.label}</p>
                </Card>
              ))}
            </div>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-[1fr_22rem]">
          <Card className="gap-3 p-5">
            <p className="font-display text-lg font-semibold text-foreground">{t.perKolomTitle}</p>
            <div className="max-h-96 overflow-auto">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-surface text-xs text-muted">
                  <tr>
                    <th className="py-2 text-left font-medium">{t.kolomColumn}</th>
                    <th className="py-2 text-right font-medium">{t.benar}</th>
                    <th className="py-2 text-right font-medium">{t.salah}</th>
                    <th className="py-2 text-right font-medium">{t.kosong}</th>
                  </tr>
                </thead>
                <tbody>
                  {result.kolom.map((k) => (
                    <tr key={k.number} className="border-t border-border">
                      <td className="py-2 font-medium text-foreground">{k.number}</td>
                      <td className="py-2 text-right font-mono text-success">{k.benar}</td>
                      <td className="py-2 text-right font-mono text-danger">{k.salah}</td>
                      <td className="py-2 text-right font-mono text-muted">{k.kosong}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          <Card className="gap-3 p-5">
            <p className="font-display text-lg font-semibold text-foreground">{t.correctPerKolomTitle}</p>
            <div className="flex h-32 items-end gap-0.5">
              {result.kolom.map((k) => {
                const total = k.benar + k.salah + k.kosong;
                return (
                  <div key={k.number} className="flex h-full min-w-0 flex-1 items-end" title={t.barLabel(k.number, k.benar)}>
                    <span className="w-full rounded-t-sm bg-accent" style={{ height: `${total > 0 ? Math.max(2, (k.benar / total) * 100) : 2}%` }} />
                  </div>
                );
              })}
            </div>
            {overview && <p className="rounded-lg bg-surface-secondary p-3 text-xs text-muted">{t.quotaLeft(overview.test.attempts_remaining)}</p>}
          </Card>
        </div>

        {completedAttempts.length > 1 && (
          <div>
            <p className="mb-2 text-sm font-semibold text-foreground">{t.historyTitle}</p>
            <div className="flex flex-wrap gap-2">
              {completedAttempts.map((attempt) => (
                <Link
                  key={attempt.session_id}
                  href={`/student/kecermatan/${testId}/result?attempt=${attempt.session_id}`}
                  aria-current={attempt.session_id === result.session_id ? "page" : undefined}
                  className={cn(
                    "rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors",
                    attempt.session_id === result.session_id
                      ? "border-accent bg-accent-soft text-accent-soft-foreground"
                      : "border-border text-muted hover:bg-surface-secondary",
                  )}
                >
                  {t.attemptLabel(attempt.attempt_number)} · {attempt.nilai_akhir}
                </Link>
              ))}
            </div>
          </div>
        )}

        {picked && column && (
          <section className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="font-display text-lg font-semibold text-foreground">{t.pembahasanTitle}</p>
              <label className="flex items-center gap-2 text-sm text-muted">
                {t.chooseKolom}
                <select
                  value={picked.number}
                  onChange={(event) => {
                    setKolomPick(Number(event.target.value));
                    setPage(0);
                  }}
                  className="rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-foreground"
                >
                  {review.kolom.map((item) => (
                    <option key={item.number} value={item.number}>
                      {t.kolomLabel(item.number)} · {t.correctOfTotal(item.benar, item.benar + item.salah + item.kosong)}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <Card className="gap-2 p-4">
              <p className="text-xs font-medium text-muted">{t.referenceTitle}</p>
              <div className="flex flex-wrap gap-2">
                {column.reference.map((item) => (
                  <div key={item.label} className="min-w-20 rounded-lg border border-border bg-surface px-2 py-2 text-center">
                    <p className="text-xs font-semibold text-muted">{item.label}</p>
                    <KecermatanItem elements={item.elements} className="mt-1 text-lg font-extrabold" elementClassName="size-5" />
                  </div>
                ))}
              </div>
            </Card>

            <div className="space-y-3">
              {column.soal.slice(from, to).map((soal, offset) => {
                const index = from + offset;
                const given = picked.answers[index] ?? null;
                const status = given === null ? "kosong" : given === soal.missing ? "benar" : "salah";
                const labelItems = (label: string | null) => column.reference.find((item) => item.label === label)?.elements ?? [];
                return (
                  <Card key={index} className="gap-3 p-4">
                    <div className="flex items-center justify-between gap-3">
                      <p className="text-sm font-semibold text-foreground">{t.soalTitle(index + 1)}</p>
                      <span className={cn("rounded-md px-2 py-0.5 text-xs font-medium", statusTone[status])}>{statusLabel[status]}</span>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-xs text-muted">{t.shownLabel}</span>
                      {soal.shown.map((label) => (
                        <span key={label} className="rounded-md border border-dashed border-border px-2 py-1">
                          <KecermatanItem elements={labelItems(label)} className="text-base font-extrabold" elementClassName="size-5" />
                        </span>
                      ))}
                    </div>
                    <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
                      <p className="flex items-center gap-2 text-muted">
                        {t.yourAnswerLabel}
                        <span className="font-mono font-semibold text-foreground">{given ?? t.blankAnswer}</span>
                        {given && <KecermatanItem elements={labelItems(given)} className="text-sm" elementClassName="size-4" />}
                      </p>
                      <p className="flex items-center gap-2 text-muted">
                        {t.correctAnswerLabel}
                        <span className="font-mono font-semibold text-foreground">{soal.missing}</span>
                        <KecermatanItem elements={labelItems(soal.missing)} className="text-sm" elementClassName="size-4" />
                      </p>
                    </div>
                  </Card>
                );
              })}
            </div>

            {pageCount > 1 && (
              <div className="flex items-center justify-between gap-3">
                <Button size="sm" variant="secondary" isDisabled={page === 0} onPress={() => setPage((current) => current - 1)}>
                  {t.prev}
                </Button>
                <span className="text-xs text-muted">{t.rangeLabel(from + 1, to, soalTotal)}</span>
                <Button size="sm" variant="secondary" isDisabled={page >= pageCount - 1} onPress={() => setPage((current) => current + 1)}>
                  {t.next}
                </Button>
              </div>
            )}
          </section>
        )}

        <div className="flex flex-wrap gap-3">
          <LinkButton href={`/student/kecermatan/${testId}`} variant="primary">
            {t.backToDetail}
          </LinkButton>
          <LinkButton href="/student" variant="secondary">
            {t.backToDashboard}
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
