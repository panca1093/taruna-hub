"use client";

import { use, useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { ArrowRight, BookOpen, CalendarClock, CheckCircle2, CloudOff, Clock, Grid2x2, RotateCcw } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { PublicNav } from "@/components/public-nav";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { Icon } from "@/components/icon";
import { Breadcrumb } from "@/components/page-heading";
import { CardKicker, NoteBox, Panel, TextLink } from "@/components/page-blocks";
import { useSession } from "@/lib/session-context";
import { useCatalogDetailT } from "@/lib/i18n/dict/catalog-detail";
import { api, ApiError, type PublicProduct } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";

type LoadState = "loading" | "ready" | "missing" | "failed";

export default function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { role } = useSession();
  const t = useCatalogDetailT();

  const [product, setProduct] = useState<PublicProduct | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);

  // A different id on the same route is not ready until its own response lands.
  const view: LoadState = state === "ready" && product?.id !== id ? "loading" : state;

  useEffect(() => {
    let cancelled = false;
    api.catalog
      .product(id)
      .then((loaded) => {
        if (cancelled) return;
        setProduct(loaded);
        setState("ready");
      })
      .catch((err) => {
        if (cancelled) return;
        setState(err instanceof ApiError && err.status === 404 ? "missing" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [id, reloadKey]);

  const body =
    view === "loading" ? (
      <div className="h-72 animate-pulse rounded-card border border-border bg-surface" />
    ) : view === "missing" ? (
      <EmptyState
        className="mt-10"
        icon={CloudOff}
        title={t.notFoundTitle}
        description={t.notFoundDescription}
        action={
          <LinkButton href="/catalog" variant="primary" size="sm">
            {t.backToCatalog}
          </LinkButton>
        }
      />
    ) : view === "failed" || product === null ? (
      <EmptyState
        className="mt-10"
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
    ) : (
      <ProductDetail product={product} />
    );

  if (role === "student" && product !== null) {
    return (
      <AppShell maxWidth="max-w-6xl">
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

function ProductDetail({ product }: { product: PublicProduct }) {
  const t = useCatalogDetailT();
  const { role } = useSession();
  const kecermatan = product.kecermatan;
  // Remaining attempts of the signed-in student; null when they hold none yet (a 404) or are not a student.
  const [kecRemaining, setKecRemaining] = useState<number | null>(null);
  const testId = kecermatan?.test_id;
  useEffect(() => {
    if (role !== "student" || !testId) return;
    let cancelled = false;
    api.kecermatanRuntime
      .overview(testId)
      .then((overview) => {
        if (!cancelled) setKecRemaining(overview.test.attempts_remaining);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [role, testId]);
  const totalSeconds = product.tryouts.reduce((sum, tryout) => sum + tryout.duration_seconds_total, 0);
  const totalMinutes = Math.round(totalSeconds / 60);

  const facts = kecermatan
    ? [
        { value: kecermatan.kolom_count, label: t.factKolom },
        { value: kecermatan.soal_per_kolom, label: t.factSoalPerKolom },
        { value: t.minutes(Math.round(kecermatan.kolom_duration_seconds / 60)), label: t.factKolomDuration },
      ]
    : [
        { value: product.tryouts.reduce((sum, tryout) => sum + tryout.subtest_count, 0), label: t.factSubtests },
        { value: product.tryouts.reduce((sum, tryout) => sum + tryout.question_count, 0), label: t.factQuestions },
        { value: t.minutes(totalMinutes), label: t.factDuration },
      ];

  return (
    <>
      <Breadcrumb items={[{ label: t.catalogCrumb, href: "/catalog" }, { label: product.title }]} />

      <div className="grid items-start gap-7 lg:grid-cols-[minmax(0,1.5fr)_minmax(17.5rem,0.65fr)]">
        <div className="flex flex-col gap-5.5">
          <Panel tone={kecermatan ? "soft" : "peach"} radius="hero" className="p-6 sm:p-7.5">
            <CardKicker icon={kecermatan ? Grid2x2 : BookOpen} className="mb-4.5">
              {kecermatan
                ? t.chipKecermatan
                : `${product.program_name} · ${product.type === "bundle" ? t.chipBundle(product.tryouts.length) : t.chipIndividual}`}
            </CardKicker>
            <h1 className="font-display mb-3 max-w-2xl text-hero-lg sm:text-display text-foreground">{product.title}</h1>
            <p className="max-w-prose text-small text-muted">{product.description}</p>
            <div className="mt-6 flex flex-wrap gap-5 sm:gap-7.5">
              {facts.map((fact) => (
                <div key={fact.label}>
                  <strong className="font-display block text-heading">{fact.value}</strong>
                  <small className="mt-1 block text-micro text-muted">{fact.label}</small>
                </div>
              ))}
            </div>
          </Panel>

          {kecermatan ? (
            <Panel>
              <h2 className="font-display mb-3 text-heading-sm">{t.howItWorksTitle}</h2>
              <p className="text-caption leading-relaxed text-muted">{t.kecermatanMechanic(kecermatan.options_per_soal)}</p>
              <div className="mt-4 flex flex-wrap gap-4 border-t border-border pt-4 text-label text-muted">
                <span>{t.kolomSoalStat(kecermatan.kolom_count, kecermatan.soal_per_kolom)}</span>
                <span>{t.perMinuteStat(Math.round(kecermatan.kolom_duration_seconds / 60))}</span>
                <span>{t.itemType[kecermatan.item_type] ?? kecermatan.item_type}</span>
              </div>
              <NoteBox className="mt-5">{t.noProctor}</NoteBox>
            </Panel>
          ) : (
            <Panel>
              <h2 className="font-display mb-3 text-heading-sm">
                {product.tryouts.length > 1 ? t.packageMultiHeading(product.tryouts.length) : t.packageSingleHeading}
              </h2>
              {product.tryouts.map((tryout, index) => (
                <div key={tryout.id} className="flex items-center gap-4 border-b border-border py-4 last:border-0">
                  <span className="grid size-8 shrink-0 place-items-center rounded-control bg-background text-label text-muted">
                    {index + 1}
                  </span>
                  <p className="min-w-0 flex-1 text-caption font-semibold text-foreground">{tryout.title}</p>
                  <span className="text-label text-muted">
                    {t.subtestStat(tryout.subtest_count, tryout.question_count)} · {t.totalMinStat(Math.round(tryout.duration_seconds_total / 60))}
                  </span>
                </div>
              ))}
            </Panel>
          )}
        </div>

        <Panel as="aside" className="lg:sticky lg:top-6">
          <p className="mb-3 text-label text-muted">{t.productLabel}</p>
          <div className="font-display text-display-sm text-foreground">{formatIDR(product.price)}</div>
          <p className="mt-1 mb-5 text-label text-muted">
            {product.original_price && <span className="mr-2 line-through">{formatIDR(product.original_price)}</span>}
            {product.discount_percent !== null && t.discountLabel(product.discount_percent)}
          </p>

          {kecermatan && kecRemaining !== null && kecRemaining > 0 ? (
            <LinkButton href={`/student/kecermatan/${kecermatan.test_id}`} variant="primary" className="w-full">
              {t.ctaKecermatanRemaining(kecRemaining)}
              <Icon icon={ArrowRight} size="xs" />
            </LinkButton>
          ) : kecermatan && kecRemaining !== null ? (
            <LinkButton href={`/checkout/${product.id}`} variant="primary" className="w-full">
              <Icon icon={RotateCcw} size="sm" /> {t.ctaKecermatanExhausted}
            </LinkButton>
          ) : (
            <LinkButton href={`/checkout/${product.id}`} variant="primary" className="w-full">
              {t.ctaBuyNow}
              <Icon icon={ArrowRight} size="xs" />
            </LinkButton>
          )}
          <TextLink href="/catalog" className="mt-2">
            {t.backToCatalog}
          </TextLink>

          <div className="mt-3 space-y-2.5 border-t border-border pt-4 text-label text-muted">
            {kecermatan ? (
              <>
                <div className="flex items-center gap-2">
                  <Icon icon={RotateCcw} size="xs" className="text-rose" />
                  {t.attemptsPerPurchase(product.included_attempts ?? 0)}
                </div>
                <div className="flex items-center gap-2">
                  <Icon icon={Clock} size="xs" className="text-rose" />
                  {t.kolomDurationStat(kecermatan.kolom_count, Math.round(kecermatan.kolom_duration_seconds / 60))}
                </div>
                {product.active_until && (
                  <div className="flex items-center gap-2">
                    <Icon icon={CalendarClock} size="xs" className="text-rose" />
                    {t.validUntil(new Date(product.active_until).toLocaleDateString())}
                  </div>
                )}
              </>
            ) : (
              <>
                {product.active_until && (
                  <div className="flex items-center gap-2">
                    <Icon icon={CalendarClock} size="xs" className="text-rose" />
                    {t.workableUntil(new Date(product.active_until).toLocaleDateString())}
                  </div>
                )}
                <div className="flex items-center gap-2">
                  <Icon icon={Clock} size="xs" className="text-rose" />
                  {t.totalDuration(totalMinutes)}
                </div>
                <div className="flex items-center gap-2">
                  <Icon icon={CheckCircle2} size="xs" className="text-success" />
                  {t.repeatable}
                </div>
              </>
            )}
          </div>
          <NoteBox className="mt-5">{t.checkoutNote}</NoteBox>
        </Panel>
      </div>
    </>
  );
}
