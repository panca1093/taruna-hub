"use client";

import { use } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Card, Chip } from "@heroui/react";
import { CalendarClock, CheckCircle2, ChevronRight, Clock, Gauge, ListChecks, RotateCcw, Timer } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { PublicNav } from "@/components/public-nav";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
import { useSession } from "@/lib/session-context";
import { remainingAttempts } from "@/lib/kecermatan-session";
import { useCatalogDetailT } from "@/lib/i18n/dict/catalog-detail";
import {
  formatDate,
  formatIDR,
  getKecermatanAttemptsPurchased,
  getKecermatanTest,
  getProduct,
  getProgram,
  getSubtestsForTryout,
  getTryout,
  hasEnrollment,
  hasKecermatanAccess,
} from "@/lib/mock-data";
import { useHydrated } from "@/lib/use-hydrated";

export default function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const product = getProduct(id);
  const { role, user } = useSession();
  const hydrated = useHydrated();
  const t = useCatalogDetailT();

  const kecermatanTest = product?.kecermatanTestId ? getKecermatanTest(product.kecermatanTestId) : undefined;

  const kecRemaining =
    hydrated && user && role === "student" && kecermatanTest
      ? remainingAttempts(kecermatanTest.id, user.id, getKecermatanAttemptsPurchased(user.id, kecermatanTest.id))
      : null;

  if (!product) notFound();

  const program = getProgram(product.programId);
  const isKecermatan = product.type === "kecermatan";
  const tryouts = product.tryoutIds.map((tid) => getTryout(tid)).filter((t) => !!t);
  const alreadyOwned = !isKecermatan && !!user && role === "student" && tryouts.every((t) => hasEnrollment(user.id, t.id));
  const totalDurationMin = tryouts.reduce(
    (sum, t) => sum + getSubtestsForTryout(t.id).reduce((s, st) => s + st.durationSeconds, 0),
    0,
  ) / 60;
  const ownsKecermatan = isKecermatan && kecermatanTest && !!user && role === "student" && hasKecermatanAccess(user.id, kecermatanTest.id);

  const body = (
    <>
        <nav className="flex items-center gap-1.5 text-xs text-muted">
          <Link href="/catalog" className="hover:text-foreground">
            {t.catalogCrumb}
          </Link>
          <ChevronRight className="size-3" />
          <span className="text-foreground">{product.title}</span>
        </nav>

        <div className="mt-5 grid gap-8 lg:grid-cols-[1.4fr_1fr]">
          <div>
            <div
              className="h-44 w-full rounded-xl sm:h-56"
              style={{ background: `linear-gradient(135deg, ${product.coverFrom}, ${product.coverTo})` }}
            />
            <div className="mt-5 flex flex-wrap items-center gap-2">
              {program && <ProgramBadge code={program.code} />}
              <Chip size="sm" variant="secondary" className="border border-border font-medium">
                {isKecermatan ? t.chipKecermatan : product.type === "bundle" ? t.chipBundle(tryouts.length) : t.chipIndividual}
              </Chip>
            </div>
            <h1 className="font-display mt-3 text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
              {product.title}
            </h1>
            <p className="mt-3 leading-relaxed text-muted">{product.description}</p>

            {isKecermatan && kecermatanTest ? (
              <div className="mt-8 space-y-4">
                <p className="text-sm font-semibold text-foreground">{t.howItWorksTitle}</p>
                <Card className="gap-3 p-4">
                  <p className="text-sm text-muted">{t.kecermatanMechanic(kecermatanTest.optionsPerSoal)}</p>
                  <div className="flex flex-wrap gap-4 border-t border-border pt-3 text-xs text-muted">
                    <span className="inline-flex items-center gap-1.5">
                      <ListChecks className="size-3.5" /> {t.kolomSoalStat(kecermatanTest.kolomCount, kecermatanTest.soalPerKolom)}
                    </span>
                    <span className="inline-flex items-center gap-1.5">
                      <Timer className="size-3.5" /> {t.perMinuteStat(Math.round(kecermatanTest.kolomDurationSeconds / 60))}
                    </span>
                    <span className="inline-flex items-center gap-1.5">
                      <Gauge className="size-3.5" /> {t.itemType[kecermatanTest.itemType]}
                    </span>
                  </div>
                </Card>
                <p className="rounded-lg border border-dashed border-border p-3 text-xs text-muted">{t.noProctor}</p>
              </div>
            ) : (
              <div className="mt-8 space-y-4">
                <p className="text-sm font-semibold text-foreground">
                  {tryouts.length > 1 ? t.packageMultiHeading(tryouts.length) : t.packageSingleHeading}
                </p>
                {tryouts.map((tryout) => {
                  const subtests = getSubtestsForTryout(tryout.id);
                  const totalQuestions = subtests.reduce((s, st) => s + st.questionIds.length, 0);
                  const totalMin = Math.round(subtests.reduce((s, st) => s + st.durationSeconds, 0) / 60);
                  return (
                    <Card key={tryout.id} className="gap-3 p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-sm font-semibold text-foreground">{tryout.title}</p>
                          <p className="mt-0.5 text-xs text-muted">{tryout.description}</p>
                        </div>
                        {tryout.scoringMode === "irt" && (
                          <Chip size="sm" className="bg-accent-soft text-accent-soft-foreground shrink-0 font-medium">
                            {t.irtModeChip}
                          </Chip>
                        )}
                      </div>
                      <div className="flex flex-wrap gap-4 border-t border-border pt-3 text-xs text-muted">
                        <span className="inline-flex items-center gap-1.5">
                          <ListChecks className="size-3.5" /> {t.subtestStat(subtests.length, totalQuestions)}
                        </span>
                        <span className="inline-flex items-center gap-1.5">
                          <Clock className="size-3.5" /> {t.totalMinStat(totalMin)}
                        </span>
                      </div>
                      <ul className="grid gap-1.5 sm:grid-cols-2">
                        {subtests.map((st) => (
                          <li key={st.id} className="flex items-center gap-2 text-xs text-muted">
                            <CheckCircle2 className="size-3.5 text-success" />
                            {t.perSubtestDuration(st.name, Math.round(st.durationSeconds / 60))}
                          </li>
                        ))}
                      </ul>
                    </Card>
                  );
                })}
              </div>
            )}
          </div>

          <div className="lg:sticky lg:top-20 lg:h-fit">
            <Card className="gap-4 p-5">
              <div className="flex items-baseline gap-2">
                <span className="font-display text-2xl font-semibold text-foreground">
                  {formatIDR(product.price)}
                </span>
                <span className="text-sm text-muted line-through">{formatIDR(product.originalPrice)}</span>
              </div>
              <p className="text-xs text-muted">
                {t.discountLabel(Math.round((1 - product.price / product.originalPrice) * 100))}
              </p>

              {isKecermatan && kecermatanTest ? (
                ownsKecermatan && kecRemaining !== null && kecRemaining > 0 ? (
                  <LinkButton href={`/student/kecermatan/${kecermatanTest.id}`} variant="primary" size="lg" className="w-full">
                    {t.ctaKecermatanRemaining(kecRemaining)}
                  </LinkButton>
                ) : ownsKecermatan ? (
                  <LinkButton href={`/checkout/${product.id}`} variant="primary" size="lg" className="w-full">
                    <RotateCcw className="size-4" /> {t.ctaKecermatanExhausted}
                  </LinkButton>
                ) : (
                  <LinkButton href={`/checkout/${product.id}`} variant="primary" size="lg" className="w-full">
                    {t.ctaBuyNow}
                  </LinkButton>
                )
              ) : alreadyOwned ? (
                <LinkButton href="/student/tryouts" variant="primary" size="lg" className="w-full">
                  {t.ctaAlreadyOwned}
                </LinkButton>
              ) : (
                <LinkButton href={`/checkout/${product.id}`} variant="primary" size="lg" className="w-full">
                  {t.ctaBuyNow}
                </LinkButton>
              )}

              <div className="space-y-2.5 border-t border-border pt-4 text-xs text-muted">
                {isKecermatan && kecermatanTest ? (
                  <>
                    <div className="flex items-center gap-2">
                      <RotateCcw className="size-3.5" />
                      {t.attemptsPerPurchase(product.includedAttempts ?? 0)}
                    </div>
                    <div className="flex items-center gap-2">
                      <Clock className="size-3.5" />
                      {t.kolomDurationStat(kecermatanTest.kolomCount, Math.round(kecermatanTest.kolomDurationSeconds / 60))}
                    </div>
                    <div className="flex items-center gap-2">
                      <CalendarClock className="size-3.5" />
                      {t.validUntil(formatDate(product.activeUntil))}
                    </div>
                  </>
                ) : (
                  <>
                    <div className="flex items-center gap-2">
                      <CalendarClock className="size-3.5" />
                      {t.workableUntil(formatDate(product.activeUntil))}
                    </div>
                    <div className="flex items-center gap-2">
                      <Clock className="size-3.5" />
                      {t.totalDuration(Math.round(totalDurationMin))}
                    </div>
                    <div className="flex items-center gap-2">
                      <ListChecks className="size-3.5" />
                      {t.repeatable}
                    </div>
                  </>
                )}
              </div>
            </Card>
          </div>
        </div>
    </>
  );

  if (role === "student") {
    return (
      <AppShell title={product.title} description={program?.name}>
        {body}
      </AppShell>
    );
  }

  return (
    <div className="min-h-dvh bg-background">
      <PublicNav />
      <div className="mx-auto max-w-5xl px-5 py-10">{body}</div>
    </div>
  );
}
