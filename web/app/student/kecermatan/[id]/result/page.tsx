"use client";

import { Suspense, use, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@heroui/react";
import { ArrowRight, CloudOff } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { Icon } from "@/components/icon";
import { KecermatanItem } from "@/components/kecermatan-item";
import { NoteBox, Panel, SectionHeader } from "@/components/page-blocks";
import { StatusChip } from "@/components/status-badge";
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
    { label: t.benar, value: result.benar_total },
    { label: t.salah, value: result.salah_total },
    { label: t.kosong, value: result.kosong_total },
  ];
  const scoreCards = [
    { label: t.scoreKecepatan, value: result.scores.kecepatan },
    { label: t.scoreKetelitian, value: result.scores.ketelitian },
    { label: t.scoreKetahanan, value: result.scores.ketahanan },
  ];
  const completedAttempts = overview?.attempts.filter((attempt) => attempt.status === "completed") ?? [];
  const completedOn = new Date(result.completed_at).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });

  const picked = review.kolom.find((item) => item.number === kolomPick) ?? review.kolom[0];
  const column = picked ? review.columns[picked.source_column - 1] : undefined;
  const soalTotal = column?.soal.length ?? 0;
  const pageCount = Math.max(1, Math.ceil(soalTotal / PAGE_SIZE));
  const from = page * PAGE_SIZE;
  const to = Math.min(soalTotal, from + PAGE_SIZE);
  const statusLabel = { benar: t.statusBenar, salah: t.statusSalah, kosong: t.statusKosong };
  const statusTone = { benar: "success", salah: "danger", kosong: "neutral" } as const;
  const statTile = "rounded-tile border border-border bg-surface p-4 sm:p-5.5";

  return (
    <AppShell
      title={t.heading}
      description={`${result.test_title} · ${completedOn}`}
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.crumbResult }]}
      actions={
        <LinkButton href="#pembahasan" variant="primary">
          {t.reviewCta} <Icon icon={ArrowRight} size="sm" />
        </LinkButton>
      }
    >
      <div className="space-y-6">
        <div className="grid gap-5 sm:gap-7.5 md:grid-cols-score">
          <Panel tone="peach" radius="hero" className="flex flex-col items-center justify-center text-center">
            <p className="text-label text-foreground/80">{t.nilaiAkhirLabel(result.attempt_number)}</p>
            <p className="my-5 font-display text-score tabular-nums">{result.scores.nilai_akhir}</p>
            <p className="text-label text-foreground/80">{t.outOf100}</p>
          </Panel>
          <div className="grid content-start gap-4">
            <div className="grid grid-cols-3 gap-2.5 sm:gap-4">
              {counts.map((item) => (
                <div key={item.label} className={statTile}>
                  <p className="font-display text-metric tabular-nums sm:text-hero">{item.value}</p>
                  <p className="mt-1.5 text-micro text-muted">{item.label}</p>
                </div>
              ))}
            </div>
            <Panel>
              <h2 className="font-display mb-3 text-heading-sm">{t.insightTitle}</h2>
              <p className="text-caption leading-loose text-muted">{t.insightBody}</p>
            </Panel>
          </div>
        </div>

        <div className="grid grid-cols-3 gap-2.5 sm:gap-4">
          {scoreCards.map((item) => (
            <div key={item.label} className={statTile}>
              <p className="font-display text-metric tabular-nums sm:text-hero">{item.value}</p>
              <p className="mt-1.5 text-micro text-muted">{item.label}</p>
            </div>
          ))}
        </div>

        <div className="grid items-start gap-7 lg:grid-cols-detail">
          <Panel>
            <h2 className="font-display mb-3 text-heading-sm">{t.perKolomTitle}</h2>
            <div className="overflow-x-auto">
              <table className="w-full min-w-80 border-collapse text-caption">
                <thead>
                  <tr>
                    <th className="border-b border-border px-2.5 py-3 text-left text-micro font-medium text-muted">{t.kolomColumn}</th>
                    <th className="border-b border-border px-2.5 py-3 text-right text-micro font-medium text-muted">{t.benar}</th>
                    <th className="border-b border-border px-2.5 py-3 text-right text-micro font-medium text-muted">{t.salah}</th>
                    <th className="border-b border-border px-2.5 py-3 text-right text-micro font-medium text-muted">{t.kosong}</th>
                  </tr>
                </thead>
                <tbody className="tabular-nums">
                  {result.kolom.map((k) => (
                    <tr key={k.number}>
                      <td className="border-b border-border px-2.5 py-3.25 text-left">{k.number}</td>
                      <td className="border-b border-border px-2.5 py-3.25 text-right">{k.benar}</td>
                      <td className="border-b border-border px-2.5 py-3.25 text-right">{k.salah}</td>
                      <td className="border-b border-border px-2.5 py-3.25 text-right">{k.kosong}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Panel>

          <Panel>
            <h2 className="font-display mb-3 text-heading-sm">{t.correctPerKolomTitle}</h2>
            <div className="mt-4 flex h-34 items-end gap-1.5 sm:gap-2.5">
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
            {overview && <NoteBox className="mt-5">{t.quotaLeft(overview.test.attempts_remaining)}</NoteBox>}
          </Panel>
        </div>

        {completedAttempts.length > 1 && (
          <div>
            <p className="mb-3 text-small font-medium">{t.historyTitle}</p>
            <div className="flex flex-wrap gap-2">
              {completedAttempts.map((attempt) => (
                <Link
                  key={attempt.session_id}
                  href={`/student/kecermatan/${testId}/result?attempt=${attempt.session_id}`}
                  aria-current={attempt.session_id === result.session_id ? "page" : undefined}
                  className={cn(
                    "inline-flex min-h-9 items-center rounded-full border px-3.5 text-label outline-none transition-colors focus-visible:ring-2 focus-visible:ring-focus",
                    attempt.session_id === result.session_id
                      ? "border-accent bg-accent text-accent-foreground"
                      : "border-border bg-surface text-muted hover:text-foreground",
                  )}
                >
                  {t.attemptLabel(attempt.attempt_number)} · {attempt.nilai_akhir}
                </Link>
              ))}
            </div>
          </div>
        )}

        {picked && column && (
          <section id="pembahasan" className="scroll-mt-6 space-y-5">
            <SectionHeader title={t.pembahasanTitle} />
            <div className="flex flex-wrap items-center gap-3">
              <label className="flex items-center gap-2.5 text-caption text-muted">
                {t.chooseKolom}
                <select
                  value={picked.number}
                  onChange={(event) => {
                    setKolomPick(Number(event.target.value));
                    setPage(0);
                  }}
                  className="min-h-10 rounded-control border border-border bg-surface px-3 text-small text-foreground outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  {review.kolom.map((item) => (
                    <option key={item.number} value={item.number}>
                      {t.kolomLabel(item.number)} · {t.correctOfTotal(item.benar, item.benar + item.salah + item.kosong)}
                    </option>
                  ))}
                </select>
              </label>
              <span className="rounded-full border border-border bg-surface px-3 py-1.5 text-label text-muted">{t.rangeLabel(from + 1, to, soalTotal)}</span>
            </div>

            <Panel tone="canvas" radius="tile">
              <p className="mb-3 text-label text-muted">{t.referenceTitle}</p>
              <div className="flex flex-wrap gap-2">
                {column.reference.map((item) => (
                  <div key={item.label} className="min-w-20 rounded-tile border border-border bg-surface px-2 py-2.5 text-center">
                    <p className="text-label text-muted">{item.label}</p>
                    <KecermatanItem elements={item.elements} className="mt-1.5 text-lg font-extrabold" elementClassName="size-5" />
                  </div>
                ))}
              </div>
            </Panel>

            <Panel>
              {column.soal.slice(from, to).map((soal, offset) => {
                const index = from + offset;
                const given = picked.answers[index] ?? null;
                const status = given === null ? "kosong" : given === soal.missing ? "benar" : "salah";
                const labelItems = (label: string | null) => column.reference.find((item) => item.label === label)?.elements ?? [];
                return (
                  <article key={index} className="border-b border-border py-6 first:pt-0 last:border-0 last:pb-0">
                    <StatusChip tone={statusTone[status]} className="mb-3">
                      {statusLabel[status]}
                    </StatusChip>
                    <h3 className="text-small font-medium">{t.soalTitle(index + 1)}</h3>
                    <div className="mt-3 flex flex-wrap items-center gap-2 text-caption text-muted">
                      {t.shownLabel}
                      {soal.shown.map((label) => (
                        <span key={label} className="rounded-chip border border-dashed border-rose px-2 py-1">
                          <KecermatanItem elements={labelItems(label)} className="text-base font-extrabold" elementClassName="size-5" />
                        </span>
                      ))}
                    </div>
                    <div className="mt-3 flex flex-wrap gap-x-6 gap-y-2 text-caption text-muted">
                      <p className="flex items-center gap-2">
                        {t.yourAnswerLabel}
                        <strong className="font-mono font-semibold text-foreground">{given ?? t.blankAnswer}</strong>
                        {given && <KecermatanItem elements={labelItems(given)} className="text-sm" elementClassName="size-4" />}
                      </p>
                      <p className="flex items-center gap-2">
                        {t.correctAnswerLabel}
                        <strong className="font-mono font-semibold text-foreground">{soal.missing}</strong>
                        <KecermatanItem elements={labelItems(soal.missing)} className="text-sm" elementClassName="size-4" />
                      </p>
                    </div>
                  </article>
                );
              })}
            </Panel>

            {pageCount > 1 && (
              <div className="flex flex-wrap items-center gap-3.5">
                <Button size="sm" variant="secondary" isDisabled={page === 0} onPress={() => setPage((current) => current - 1)}>
                  {t.prev}
                </Button>
                <Button size="sm" variant="primary" isDisabled={page >= pageCount - 1} onPress={() => setPage((current) => current + 1)}>
                  {t.next}
                </Button>
              </div>
            )}
          </section>
        )}

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
