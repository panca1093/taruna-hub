"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Card, Chip } from "@heroui/react";
import {
  Award,
  BadgePercent,
  BarChart3,
  ClipboardCheck,
  FileDown,
  FileSearch,
  ShoppingCart,
  Sparkles,
  Timer,
} from "lucide-react";
import { PublicNav } from "@/components/public-nav";
import { HeroExamPreview } from "@/components/marketing/hero-exam-preview";
import { LinkButton } from "@/components/ui/link-button";
import { api, type CatalogSummary, type PublicProduct, type PublicProgram } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { useLandingT } from "@/lib/i18n/dict/landing";

const valuePropIcons = [Sparkles, Timer, Award, FileDown, BarChart3, BadgePercent];

const stepIcons = [ShoppingCart, ClipboardCheck, FileSearch];

export default function HomePage() {
  const t = useLandingT();
  const [featured, setFeatured] = useState<PublicProduct[]>([]);
  const [summary, setSummary] = useState<CatalogSummary | null>(null);
  const [programs, setPrograms] = useState<PublicProgram[] | null>(null);

  // The landing reads only the public catalog; a failure leaves the numbers empty, never invented.
  useEffect(() => {
    let cancelled = false;
    api.catalog
      .products({ limit: 4 })
      .then((page) => {
        if (!cancelled) setFeatured(page.products);
      })
      .catch(() => {
        if (!cancelled) setFeatured([]);
      });
    api.catalog
      .summary()
      .then((counts) => {
        if (!cancelled) setSummary(counts);
      })
      .catch(() => {
        if (!cancelled) setSummary(null);
      });
    api.catalog
      .programs()
      .then((list) => {
        if (!cancelled) setPrograms(list.programs);
      })
      .catch(() => {
        if (!cancelled) setPrograms([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="min-h-dvh bg-background">
      <PublicNav />

      {/* Hero */}
      <section className="relative overflow-hidden bg-grid">
        <div className="pointer-events-none absolute inset-x-0 top-0 h-[480px] bg-gradient-to-b from-accent-soft/60 to-transparent" />
        <div className="relative mx-auto grid max-w-6xl gap-12 px-5 py-20 lg:grid-cols-[1.05fr_0.95fr] lg:items-center lg:py-28">
          <div>
            <Chip size="sm" className="bg-accent-soft text-accent-soft-foreground mb-5 font-medium">
              {t.heroBadge}
            </Chip>
            <h1 className="font-display max-w-xl text-4xl font-semibold tracking-tight text-balance text-foreground sm:text-5xl">
              {t.heroTitle}
            </h1>
            <p className="mt-5 max-w-lg text-base leading-relaxed text-muted sm:text-lg">{t.heroDescription}</p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <LinkButton href="/register" variant="primary" size="lg">
                {t.ctaRegister}
              </LinkButton>
              <LinkButton href="/catalog" variant="ghost" size="lg">
                {t.ctaCatalog}
              </LinkButton>
            </div>
            <dl className="mt-12 grid max-w-md grid-cols-3 gap-6 border-t border-border pt-6">
              <div>
                <dt className="font-display text-2xl font-semibold text-foreground">{summary?.programs ?? 0}</dt>
                <dd className="text-xs text-muted">{t.statPrograms}</dd>
              </div>
              <div>
                <dt className="font-display text-2xl font-semibold text-foreground">{summary?.tryouts ?? 0}</dt>
                <dd className="text-xs text-muted">{t.statTryouts}</dd>
              </div>
              <div>
                <dt className="font-display text-2xl font-semibold text-foreground">{t.statInstant}</dt>
                <dd className="text-xs text-muted">{t.statScoreDiscussion}</dd>
              </div>
            </dl>
          </div>
          <HeroExamPreview />
        </div>
      </section>

      {/* Value props */}
      <section className="bg-surface-secondary/60">
        <div className="mx-auto max-w-6xl px-5 py-16">
          <p className="text-sm font-medium text-muted">{t.valuePropsEyebrow}</p>
          <h2 className="font-display mt-1 max-w-md text-2xl font-semibold tracking-tight text-foreground">
            {t.valuePropsHeading}
          </h2>
          <div className="mt-10 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {valuePropIcons.map((Icon, i) => {
              const copy = t.valueProps[i];
              return (
                <Card key={copy.title} className="gap-3 p-5">
                  <span className="inline-flex size-10 items-center justify-center rounded-lg bg-gradient-ring text-white">
                    <Icon className="size-5" strokeWidth={2} />
                  </span>
                  <p className="text-[15px] font-semibold text-foreground">{copy.title}</p>
                  <p className="text-sm leading-relaxed text-muted">{copy.description}</p>
                </Card>
              );
            })}
          </div>
        </div>
      </section>

      {/* Programs */}
      <section id="program" className="mx-auto max-w-6xl px-5 py-16">
        <p className="text-sm font-medium text-muted">{t.programsEyebrow}</p>
        <h2 className="font-display mt-1 text-2xl font-semibold tracking-tight text-foreground">{t.programsHeading}</h2>
        {programs?.length === 0 ? (
          <p className="mt-8 text-sm text-muted">{t.programsEmpty}</p>
        ) : (
          <div className="mt-8 grid gap-5 sm:grid-cols-3">
            {(programs ?? []).map((program, i) => (
              <Link key={program.id} href={`/catalog?program=${encodeURIComponent(program.code)}`}>
                <Card className="group h-full gap-3 p-5 transition-shadow hover:shadow-overlay">
                  <span className="font-mono inline-flex size-10 items-center justify-center rounded-full bg-gradient-ring text-sm font-semibold text-white">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <p className="text-[15px] font-semibold text-foreground">{program.name}</p>
                  {program.description && <p className="text-sm leading-relaxed text-muted">{program.description}</p>}
                  <span className="mt-auto text-sm font-medium text-accent group-hover:underline">{t.diklatCta}</span>
                </Card>
              </Link>
            ))}
          </div>
        )}
      </section>

      {/* Featured products */}
      <section className="mx-auto max-w-6xl px-5 py-16">
        <div className="flex items-end justify-between gap-4">
          <div>
            <p className="text-sm font-medium text-muted">{t.featuredEyebrow}</p>
            <h2 className="font-display mt-1 text-2xl font-semibold tracking-tight text-foreground">
              {t.featuredHeading}
            </h2>
          </div>
          <Link href="/catalog" className="hidden text-sm font-medium text-accent hover:underline sm:block">
            {t.featuredViewAll}
          </Link>
        </div>
        {featured.length === 0 ? (
          <p className="mt-8 text-sm text-muted">{t.featuredEmpty}</p>
        ) : (
          <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {featured.map((product) => (
              <Link key={product.id} href={`/catalog/${product.id}`}>
                <Card className="group h-full gap-0 overflow-hidden p-0 transition-shadow hover:shadow-overlay">
                  <div className="relative h-28 w-full bg-gradient-ring">
                    <Chip size="sm" className="absolute top-3 left-3 bg-white/20 text-white backdrop-blur-sm">
                      {product.program_code}
                    </Chip>
                  </div>
                  <div className="space-y-2 p-4">
                    <p className="line-clamp-1 text-sm font-semibold text-foreground">{product.title}</p>
                    <div className="flex items-baseline gap-2">
                      <span className="font-display text-base font-semibold text-foreground">
                        {formatIDR(product.price)}
                      </span>
                      {product.original_price && (
                        <span className="text-xs text-muted line-through">{formatIDR(product.original_price)}</span>
                      )}
                    </div>
                  </div>
                </Card>
              </Link>
            ))}
          </div>
        )}
      </section>

      {/* How it works */}
      <section id="cara-kerja" className="bg-surface-secondary/60">
        <div className="mx-auto max-w-6xl px-5 py-16">
          <p className="text-sm font-medium text-muted">{t.howItWorksEyebrow}</p>
          <h2 className="font-display mt-1 max-w-md text-2xl font-semibold tracking-tight text-foreground">
            {t.howItWorksHeading}
          </h2>
          <div className="relative mt-10 grid gap-8 sm:grid-cols-3">
            <div className="absolute top-5 right-[16.5%] left-[16.5%] hidden h-px bg-border sm:block" aria-hidden />
            {stepIcons.map((Icon, i) => {
              const copy = t.steps[i];
              return (
                <div key={copy.title} className="relative">
                  <div className="flex items-center gap-3">
                    <span className="font-mono relative z-10 flex size-10 shrink-0 items-center justify-center rounded-full bg-gradient-ring text-sm font-semibold text-white">
                      {String(i + 1).padStart(2, "0")}
                    </span>
                    <Icon className="size-4 text-muted" />
                  </div>
                  <p className="mt-3 text-[15px] font-semibold text-foreground">{copy.title}</p>
                  <p className="mt-1 text-sm leading-relaxed text-muted">{copy.description}</p>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="mx-auto max-w-6xl px-5 py-16">
        <Card className="items-center gap-4 bg-gradient-ring p-10 text-center text-white">
          <h2 className="font-display text-2xl font-semibold tracking-tight sm:text-3xl">{t.ctaHeading}</h2>
          <p className="max-w-md text-sm text-white/85 sm:text-base">{t.ctaDescription}</p>
          <LinkButton href="/register" variant="secondary" size="lg" className="mt-2 bg-white text-accent hover:bg-white/90">
            {t.ctaRegister}
          </LinkButton>
        </Card>
      </section>

      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-6xl flex-col items-center gap-3 px-5 py-8 text-sm text-muted sm:flex-row sm:justify-between">
          <div>
            <p>{t.footerRights}</p>
            <p className="text-xs text-muted/70">{t.footerPoweredBy}</p>
          </div>
          <p>{t.footerTagline}</p>
        </div>
      </footer>
    </div>
  );
}
