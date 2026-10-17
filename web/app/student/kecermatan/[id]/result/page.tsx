"use client";

import { use, useEffect } from "react";
import { notFound, useRouter } from "next/navigation";
import { Accordion, Card, Chip } from "@heroui/react";
import { CheckCircle2, ChevronDown, Gauge, MinusCircle, Timer, Trophy, XCircle } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CountdownRing } from "@/components/countdown-ring";
import { useUser } from "@/lib/session-context";
import { computeResult, loadSession } from "@/lib/kecermatan-session";
import { getKecermatanTest } from "@/lib/mock-data";
import { cn } from "@/lib/cn";
import { useKecermatanResultT } from "@/lib/i18n/dict/kecermatan-result";
import { useHydrated } from "@/lib/use-hydrated";

export default function KecermatanResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const user = useUser();
  const t = useKecermatanResultT();
  const test = getKecermatanTest(id);
  const hydrated = useHydrated();
  const loaded = hydrated && test ? loadSession(test.id, user.id) : null;
  const session = loaded?.status === "completed" ? loaded : null;
  const result = test && session ? computeResult(test, session) : null;
  const mustRedirect = hydrated && !!test && !session;

  useEffect(() => {
    if (mustRedirect) router.replace(`/student/kecermatan/${id}`);
  }, [mustRedirect, router, id]);

  if (!test) notFound();
  if (!result || !session) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const scoreCards = [
    { label: t.scoreKecepatan, value: result.kecepatanScore, icon: Timer },
    { label: t.scoreKetahanan, value: result.ketahananScore, icon: Gauge },
    { label: t.scoreKetelitian, value: result.ketelitianScore, icon: CheckCircle2 },
  ];

  return (
    <AppShell title={t.pageTitle} description={test.title}>
      <div className="mx-auto max-w-3xl space-y-6">
        <Card className="flex-col items-center gap-4 p-8 text-center sm:flex-row sm:text-left">
          <CountdownRing value={result.nilaiAkhirScore} size={110} strokeWidth={9} animate>
            <div className="text-center">
              <p className="font-display text-2xl font-semibold text-foreground">{result.nilaiAkhirScore}</p>
              <p className="text-[10px] text-muted">{t.outOf100}</p>
            </div>
          </CountdownRing>
          <div className="flex-1">
            <p className="text-sm font-medium text-muted">{t.nilaiAkhirLabel(result.attemptNumber)}</p>
            <p className="font-display mt-0.5 text-xl font-semibold text-foreground">{test.title}</p>
            <div className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent-soft-foreground">
              <Trophy className="size-3.5" />
              {t.scoreSummary(result.benarTotal, result.salahTotal, result.kosongTotal)}
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
            {result.kolomResults.map((k) => (
              <Card key={k.kolomNumber} className="gap-2 p-4">
                <p className="text-xs font-medium text-muted">{t.kolomLabel(k.kolomNumber)}</p>
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
            {result.kolomResults.map((k) => {
              const soalList = session.soalByKolom[k.kolomNumber] ?? [];
              return (
                <Accordion.Item key={k.kolomNumber} id={String(k.kolomNumber)} className="rounded-xl border border-border bg-surface px-4">
                  <Accordion.Heading>
                    <Accordion.Trigger className="flex w-full items-center justify-between gap-3 py-3.5 text-left">
                      <span className="text-sm font-medium text-foreground">{t.kolomLabel(k.kolomNumber)}</span>
                      <span className="flex items-center gap-2 text-xs text-muted">
                        {t.correctOfTotal(k.benar, test.soalPerKolom)}
                        <Accordion.Indicator>
                          <ChevronDown className="size-4" />
                        </Accordion.Indicator>
                      </span>
                    </Accordion.Trigger>
                  </Accordion.Heading>
                  <Accordion.Panel>
                    <Accordion.Body className="flex flex-col gap-3 border-t border-border pb-4 pt-4">
                      {soalList.map((soal, i) => {
                        const status = soal.selectedLabel === null ? "kosong" : soal.isCorrect ? "benar" : "salah";
                        return (
                          <div key={soal.soalNumber} className="space-y-2 border-b border-border pb-3 last:border-b-0 last:pb-0">
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
                                {t.missingItemLabel}<span className="font-mono font-semibold">{soal.missingLabel}</span>{" "}
                                <span className="font-mono text-muted">
                                  ({soal.referenceSet.find((r) => r.label === soal.missingLabel)?.value})
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
                                {t.yourAnswer(soal.selectedLabel)}
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
