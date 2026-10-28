"use client";

import { Suspense, use, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Accordion, Button, Card, Chip } from "@heroui/react";
import { CheckCircle2, ChevronDown, CloudOff, Gauge, MinusCircle, Timer, Trophy, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { EmptyState } from "@/components/empty-state";
import { KecermatanItem } from "@/components/kecermatan-item";
import { ApiError, api, type KecermatanReview } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useKecermatanResultT } from "@/lib/i18n/dict/kecermatan-result";

type LoadState = "loading" | "ready" | "failed";

function ResultContent({ testId }: { testId: string }) {
  const router = useRouter();
  const t = useKecermatanResultT();
  const attemptParam = useSearchParams().get("attempt");
  const [review, setReview] = useState<KecermatanReview | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      let sessionId = attemptParam;
      if (!sessionId) {
        const overview = await api.kecermatanRuntime.overview(testId);
        sessionId = overview.attempts.find((attempt) => attempt.status === "completed")?.session_id ?? null;
      }
      if (!sessionId) throw new ApiError(404, { code: "not_found", message: "" });
      const [result, detail] = await Promise.all([
        api.kecermatanRuntime.result(sessionId),
        api.kecermatanRuntime.review(sessionId),
      ]);
      return { ...detail, result: { ...detail.result, kolom: result.kolom } };
    };
    load()
      .then((data) => {
        if (cancelled) return;
        setReview(data);
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
        <p className="text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const { result } = review;
  const scoreCards = [
    { label: t.scoreKecepatan, value: result.scores.kecepatan, icon: Timer },
    { label: t.scoreKetahanan, value: result.scores.ketahanan, icon: Gauge },
    { label: t.scoreKetelitian, value: result.scores.ketelitian, icon: CheckCircle2 },
  ];
  const soalPerKolom = review.columns[0]?.soal.length ?? 0;

  return (
    <AppShell title={t.pageTitle} description={result.test_title}>
      <div className="mx-auto max-w-3xl space-y-6">
        <Card className="flex-col items-center gap-4 p-8 text-center sm:flex-row sm:text-left">
          <CountdownRing value={result.scores.nilai_akhir} size={110} strokeWidth={9} animate>
            <div className="text-center">
              <p className="font-display text-2xl font-semibold text-foreground">{result.scores.nilai_akhir}</p>
              <p className="text-[10px] text-muted">{t.outOf100}</p>
            </div>
          </CountdownRing>
          <div className="flex-1">
            <p className="text-sm font-medium text-muted">{t.nilaiAkhirLabel(result.attempt_number)}</p>
            <p className="font-display mt-0.5 text-xl font-semibold text-foreground">{result.test_title}</p>
            <div className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent-soft-foreground">
              <Trophy className="size-3.5" />
              {t.scoreSummary(result.benar_total, result.salah_total, result.kosong_total)}
            </div>
          </div>
        </Card>

        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.compositeScoreTitle}</p>
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

        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.perKolomTitle}</p>
          <div className="grid gap-3 sm:grid-cols-3">
            {result.kolom.map((k) => (
              <Card key={k.number} className="gap-2 p-4">
                <p className="text-xs font-medium text-muted">{t.kolomLabel(k.number)}</p>
                <div className="flex items-baseline gap-3 text-sm">
                  <span className="font-mono font-semibold text-success">{k.benar}</span>
                  <span className="font-mono font-semibold text-danger">{k.salah}</span>
                  <span className="font-mono font-semibold text-muted">{k.kosong}</span>
                </div>
                <p className="text-[11px] text-muted">{t.benarSalahKosong}</p>
              </Card>
            ))}
          </div>
        </div>

        <div>
          <p className="mb-3 text-sm font-semibold text-foreground">{t.pembahasanTitle}</p>
          <Accordion className="flex flex-col gap-3" allowsMultipleExpanded>
            {review.kolom.map((k) => {
              const column = review.columns[k.source_column - 1];
              return (
                <Accordion.Item key={k.number} id={String(k.number)} className="rounded-xl border border-border bg-surface px-4">
                  <Accordion.Heading>
                    <Accordion.Trigger className="flex w-full items-center justify-between gap-3 py-3.5 text-left">
                      <span className="text-sm font-medium text-foreground">{t.kolomLabel(k.number)}</span>
                      <span className="flex items-center gap-2 text-xs text-muted">
                        {t.correctOfTotal(k.benar, soalPerKolom)}
                        <Accordion.Indicator>
                          <ChevronDown className="size-4" />
                        </Accordion.Indicator>
                      </span>
                    </Accordion.Trigger>
                  </Accordion.Heading>
                  <Accordion.Panel>
                    <Accordion.Body className="flex flex-col gap-3 border-t border-border pb-4 pt-4">
                      {column.soal.map((soal, i) => {
                        const given = k.answers[i];
                        const status = given === null || given === undefined ? "kosong" : given === soal.missing ? "benar" : "salah";
                        const missing = column.reference.find((r) => r.label === soal.missing);
                        return (
                          <div key={i} className="space-y-2 border-b border-border pb-3 last:border-b-0 last:pb-0">
                            <div className="flex items-start gap-2.5">
                              <span
                                className={cn(
                                  "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full",
                                  status === "benar"
                                    ? "bg-success-soft text-success-soft-foreground"
                                    : status === "salah"
                                      ? "bg-danger-soft text-danger-soft-foreground"
                                      : "bg-default text-default-foreground",
                                )}
                              >
                                {status === "benar" ? (
                                  <CheckCircle2 className="size-3.5" />
                                ) : status === "salah" ? (
                                  <XCircle className="size-3.5" />
                                ) : (
                                  <MinusCircle className="size-3.5" />
                                )}
                              </span>
                              <p className="text-sm text-foreground">
                                <span className="text-muted">{t.soalIndex(i + 1)}</span>
                                {t.missingItemLabel}
                                <span className="font-mono font-semibold">{soal.missing}</span>{" "}
                                <span className="text-muted">
                                  (<KecermatanItem elements={missing?.elements ?? []} className="text-muted" />)
                                </span>
                              </p>
                            </div>
                            <div className="ml-7.5 flex flex-wrap gap-2 text-xs">
                              <Chip
                                size="sm"
                                className={cn(
                                  "font-medium",
                                  status === "benar"
                                    ? "bg-success-soft text-success-soft-foreground"
                                    : status === "salah"
                                      ? "bg-danger-soft text-danger-soft-foreground"
                                      : "bg-default text-default-foreground",
                                )}
                              >
                                {t.yourAnswer(given ?? null)}
                              </Chip>
                            </div>
                          </div>
                        );
                      })}
                    </Accordion.Body>
                  </Accordion.Panel>
                </Accordion.Item>
              );
            })}
          </Accordion>
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
