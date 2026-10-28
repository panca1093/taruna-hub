"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip } from "@heroui/react";
import { CalendarClock, CheckCircle2, ChevronRight, CloudOff, Clock, Gauge, ListChecks, RotateCcw, Timer } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { PublicNav } from "@/components/public-nav";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { ProgramBadge } from "@/components/status-badge";
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
      <div className="mt-8 h-72 animate-pulse rounded-xl border border-border bg-surface-secondary" />
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
      <AppShell title={product.title} description={product.program_name}>
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

  return (
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
          <div className="h-44 w-full rounded-xl bg-gradient-ring sm:h-56" />
          <div className="mt-5 flex flex-wrap items-center gap-2">
            <ProgramBadge code={product.program_code} />
            <Chip size="sm" variant="secondary" className="border border-border font-medium">
              {kecermatan
                ? t.chipKecermatan
                : product.type === "bundle"
                  ? t.chipBundle(product.tryouts.length)
                  : t.chipIndividual}
            </Chip>
          </div>
          <h1 className="font-display mt-3 text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
            {product.title}
          </h1>
          <p className="mt-3 leading-relaxed text-muted">{product.description}</p>

          {kecermatan ? (
            <div className="mt-8 space-y-4">
              <p className="text-sm font-semibold text-foreground">{t.howItWorksTitle}</p>
              <Card className="gap-3 p-4">
                <p className="text-sm text-muted">{t.kecermatanMechanic(kecermatan.options_per_soal)}</p>
                <div className="flex flex-wrap gap-4 border-t border-border pt-3 text-xs text-muted">
                  <span className="inline-flex items-center gap-1.5">
                    <ListChecks className="size-3.5" /> {t.kolomSoalStat(kecermatan.kolom_count, kecermatan.soal_per_kolom)}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <Timer className="size-3.5" /> {t.perMinuteStat(Math.round(kecermatan.kolom_duration_seconds / 60))}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <Gauge className="size-3.5" /> {t.itemType[kecermatan.item_type] ?? kecermatan.item_type}
                  </span>
                </div>
              </Card>
              <p className="rounded-lg border border-dashed border-border p-3 text-xs text-muted">{t.noProctor}</p>
            </div>
          ) : (
          <div className="mt-8 space-y-4">
            <p className="text-sm font-semibold text-foreground">
              {product.tryouts.length > 1 ? t.packageMultiHeading(product.tryouts.length) : t.packageSingleHeading}
            </p>
            {product.tryouts.map((tryout) => (
              <Card key={tryout.id} className="gap-3 p-4">
                <p className="text-sm font-semibold text-foreground">{tryout.title}</p>
                <div className="flex flex-wrap gap-4 border-t border-border pt-3 text-xs text-muted">
                  <span className="inline-flex items-center gap-1.5">
                    <ListChecks className="size-3.5" /> {t.subtestStat(tryout.subtest_count, tryout.question_count)}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <Timer className="size-3.5" /> {t.totalMinStat(Math.round(tryout.duration_seconds_total / 60))}
                  </span>
                </div>
              </Card>
            ))}
          </div>
          )}
        </div>

        <div className="lg:sticky lg:top-20 lg:h-fit">
          <Card className="gap-4 p-5">
            <div className="flex items-baseline gap-2">
              <span className="font-display text-2xl font-semibold text-foreground">{formatIDR(product.price)}</span>
              {product.original_price && (
                <span className="text-sm text-muted line-through">{formatIDR(product.original_price)}</span>
              )}
            </div>
            {product.discount_percent !== null && (
              <p className="text-xs text-muted">{t.discountLabel(product.discount_percent)}</p>
            )}

            {kecermatan && kecRemaining !== null && kecRemaining > 0 ? (
              <LinkButton href={`/student/kecermatan/${kecermatan.test_id}`} variant="primary" size="lg" className="w-full">
                {t.ctaKecermatanRemaining(kecRemaining)}
              </LinkButton>
            ) : kecermatan && kecRemaining !== null ? (
              <LinkButton href={`/checkout/${product.id}`} variant="primary" size="lg" className="w-full">
                <RotateCcw className="size-4" /> {t.ctaKecermatanExhausted}
              </LinkButton>
            ) : (
              <LinkButton href={`/checkout/${product.id}`} variant="primary" size="lg" className="w-full">
                {t.ctaBuyNow}
              </LinkButton>
            )}
            <p className="text-xs text-muted">{t.checkoutNote}</p>

            <div className="space-y-2.5 border-t border-border pt-4 text-xs text-muted">
              {kecermatan ? (
                <>
                  <div className="flex items-center gap-2">
                    <RotateCcw className="size-3.5" />
                    {t.attemptsPerPurchase(product.included_attempts ?? 0)}
                  </div>
                  <div className="flex items-center gap-2">
                    <Clock className="size-3.5" />
                    {t.kolomDurationStat(kecermatan.kolom_count, Math.round(kecermatan.kolom_duration_seconds / 60))}
                  </div>
                  {product.active_until && (
                    <div className="flex items-center gap-2">
                      <CalendarClock className="size-3.5" />
                      {t.validUntil(new Date(product.active_until).toLocaleDateString())}
                    </div>
                  )}
                </>
              ) : (
                <>
                  {product.active_until && (
                    <div className="flex items-center gap-2">
                      <CalendarClock className="size-3.5" />
                      {t.workableUntil(new Date(product.active_until).toLocaleDateString())}
                    </div>
                  )}
                  <div className="flex items-center gap-2">
                    <Clock className="size-3.5" />
                    {t.totalDuration(totalMinutes)}
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="size-3.5 text-success" />
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
}
