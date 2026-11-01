import Link from "next/link";
import { ArrowRight, BookOpen, Plus, ShieldCheck, Star } from "lucide-react";
import { Button } from "@heroui/react";
import { Logo } from "../logo";
import { LinkButton } from "../ui/link-button";
import { HeroExamPreview, programSlots, type ProgramSlot } from "./hero-exam-preview";
import type { PublicProduct, WebsiteContent } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { formatIDR } from "@/lib/format-money";
import { landingDict, type LandingDict } from "@/lib/i18n/dict/landing";
import type { Locale } from "@/lib/i18n/locale-context";

export type WebsiteLocaleContent = WebsiteContent["locales"]["id"];
export type Remote<T> = { status: "loading" } | { status: "error" } | { status: "ready"; data: T };

/** "home" only for now; program detail views extend this union and the switch in WebsiteView. */
export type WebsiteViewName = "home";

export interface WebsiteViewProps {
  /** null means no published content: loading, missing or failed; `contentState` says which. */
  content: WebsiteLocaleContent | null;
  contentState: "loading" | "ready" | "unavailable";
  locale: Locale;
  mode: "published" | "draft";
  view: WebsiteViewName;
  catalog: { products: Remote<PublicProduct[]> };
  onRetryContent?: () => void;
  onRetryCatalog?: () => void;
  /** Draft mode only: the link back to the editor. */
  draftAction?: React.ReactNode;
}

const sectionPad = "px-5 py-16 sm:px-10 sm:py-20";
const wrap = "mx-auto w-full max-w-landing";

function Heading({ title, body, className }: { title: string; body?: string; className?: string }) {
  return (
    <div className={cn("mb-8 flex flex-col gap-4 sm:mb-9 md:flex-row md:items-end md:justify-between md:gap-12", className)}>
      <h2 className="font-display text-landing-h2 text-balance text-foreground">{title}</h2>
      {body && <p className="max-w-md text-small leading-relaxed text-muted">{body}</p>}
    </div>
  );
}

function HomeView({ content, contentState, mode, catalog, t, onRetryContent, onRetryCatalog, draftAction }: WebsiteViewProps & { t: LandingDict }) {
  const home = content?.home;
  const loading = contentState === "loading";
  const programTitles = Object.fromEntries(
    programSlots.map((slot) => [slot, content?.programs[slot].title ?? t.programSlots[slot]]),
  ) as Record<ProgramSlot, string>;

  return (
    <>
      {mode === "draft" && (
        <div role="status" className="border-b border-border bg-accent-soft px-5 py-3 text-caption text-accent-soft-foreground sm:px-10">
          <div className={cn(wrap, "flex flex-wrap items-center justify-between gap-3")}>
            <p>
              <strong className="font-semibold">{t.draftBanner}.</strong> {t.draftBannerBody}
            </p>
            {draftAction}
          </div>
        </div>
      )}

      <main id="main">
        <section className="px-5 sm:px-10">
          <div className={cn(wrap, "grid items-center gap-10 py-12 md:py-20 lg:grid-cols-2 lg:gap-16")}>
            <div className="min-w-0">
              <p className="mb-6 flex items-center gap-2.5 text-label text-muted">
                <span aria-hidden className="h-px w-8 bg-rose" />
                {t.heroKicker}
              </p>
              {loading ? (
                <div aria-busy="true" className="space-y-3">
                  <div className="h-14 w-4/5 rounded-control bg-surface-secondary motion-safe:animate-pulse" />
                  <div className="h-14 w-3/5 rounded-control bg-surface-secondary motion-safe:animate-pulse" />
                  <div className="mt-6 h-20 max-w-md rounded-control bg-surface-secondary motion-safe:animate-pulse" />
                </div>
              ) : (
                <>
                  <h1 className="font-display text-landing-hero max-w-hero-title text-balance break-words text-foreground">
                    {home?.hero_title ?? t.fallbackHeroTitle}
                  </h1>
                  <p className="mt-6 max-w-md text-base leading-relaxed text-muted">{home?.hero_body ?? t.fallbackHeroBody}</p>
                </>
              )}
              <div className="mt-8 grid max-w-md grid-cols-2 gap-3">
                <LinkButton href="/#programs" variant="primary" size="lg" className="h-auto min-h-12 min-w-0 text-center break-words">
                  {home?.program_cta ?? t.fallbackProgramCta}
                  <ArrowRight className="size-4 shrink-0" aria-hidden />
                </LinkButton>
                <LinkButton href="/catalog" variant="primary" size="lg" className="h-auto min-h-12 min-w-0 text-center break-words">
                  {home?.practice_cta ?? t.fallbackPracticeCta}
                  <ArrowRight className="size-4 shrink-0" aria-hidden />
                </LinkButton>
              </div>
            </div>
            <HeroExamPreview t={t} programTitles={programTitles} />
          </div>

          <ul className={cn(wrap, "flex flex-col justify-between gap-3 border-y border-border py-5 sm:flex-row sm:gap-5")}>
            {t.facts.map((fact) => (
              <li key={fact} className="flex items-center gap-2.5 text-caption text-muted">
                <ShieldCheck className="size-4 text-accent" aria-hidden />
                {fact}
              </li>
            ))}
          </ul>
        </section>

        <section className={sectionPad}>
          <div className={wrap}>
            <Heading title={t.chooseHeading} body={t.chooseBody} />
            <div className="grid gap-6 md:grid-cols-2">
              <Link
                href="/#programs"
                className="group flex min-h-80 flex-col rounded-hero border border-border bg-surface p-7 sm:p-8"
              >
                <span className="mb-8 flex items-center gap-2 text-label font-medium text-accent">
                  <BookOpen className="size-4" aria-hidden />
                  {t.entryProgramKicker}
                </span>
                <h3 className="font-display text-display-sm text-foreground">{t.entryProgramHeading}</h3>
                <p className="mt-3 max-w-md text-small leading-relaxed text-muted">{t.entryProgramBody}</p>
                <p className="mt-5 text-micro text-muted">{t.entryProgramMeta}</p>
                <span className="mt-auto flex items-center gap-2 pt-6 text-caption font-medium text-accent group-hover:underline">
                  {t.entryProgramCta}
                  <ArrowRight className="size-4" aria-hidden />
                </span>
              </Link>
              <Link href="/catalog" className="group flex min-h-80 flex-col rounded-hero border border-accent-soft bg-accent-soft p-7 sm:p-8">
                <span className="mb-8 flex items-center gap-2 text-label font-medium text-accent-soft-foreground">
                  <ShieldCheck className="size-4" aria-hidden />
                  {t.entryPracticeKicker}
                </span>
                <h3 className="font-display text-display-sm text-foreground">{t.entryPracticeHeading}</h3>
                <p className="mt-3 max-w-md text-small leading-relaxed text-foreground">{t.entryPracticeBody}</p>
                <p className="mt-5 text-micro text-foreground">{t.entryPracticeMeta}</p>
                <span className="mt-auto flex items-center gap-2 pt-6 text-caption font-medium text-accent-soft-foreground group-hover:underline">
                  {t.entryPracticeCta}
                  <ArrowRight className="size-4" aria-hidden />
                </span>
              </Link>
            </div>
          </div>
        </section>

        <section id="programs" className="scroll-mt-24 px-5 pb-16 sm:px-10 sm:pb-20">
          <div className={wrap}>
            <Heading title={t.programsHeading} body={t.programsBody} />
            {contentState === "unavailable" ? (
              <div role="status" className="rounded-card border border-border bg-surface p-6">
                <p className="text-title text-foreground">{t.programsUnavailableTitle}</p>
                <p className="mt-2 max-w-lg text-small text-muted">{t.programsUnavailableBody}</p>
                {onRetryContent && (
                  <Button variant="secondary" size="sm" className="mt-4" onPress={onRetryContent}>
                    {t.programsRetry}
                  </Button>
                )}
              </div>
            ) : (
              <div className="grid gap-5 md:grid-cols-3" aria-busy={loading}>
                {programSlots.map((slot) => {
                  const program = content?.programs[slot];
                  if (!program) return <div key={slot} className="h-64 rounded-card bg-surface-secondary motion-safe:animate-pulse" />;
                  const rows = [
                    [t.programScheduleLabel, program.schedule],
                    [t.programLocationLabel, program.location],
                    [t.programFeesLabel, program.fees],
                  ] as const;
                  return (
                    <Link key={slot} href={`/programs/${slot}`} className="group flex min-h-64 flex-col rounded-card border border-border bg-surface p-6">
                      <span className="mb-5 flex size-11 items-center justify-center rounded-control bg-peach text-accent">
                        {slot === "kedinasan" ? <BookOpen className="size-5" aria-hidden /> : <ShieldCheck className="size-5" aria-hidden />}
                      </span>
                      <p className="mb-2 text-micro text-muted">{t.programKicker}</p>
                      <h3 className="font-display text-landing-h3 break-words text-foreground">{program.title}</h3>
                      <p className="mt-3 text-label leading-relaxed break-words text-muted">{program.summary}</p>
                      <dl className="mt-4 space-y-1.5 text-micro">
                        {rows.map(([label, value]) => (
                          <div key={label} className="flex gap-2">
                            <dt className="shrink-0 text-muted">{label}</dt>
                            <dd className="min-w-0 break-words text-foreground">{value.trim() === "" ? t.programPending : value}</dd>
                          </div>
                        ))}
                      </dl>
                      <span className="mt-auto flex items-center gap-2 pt-5 text-caption font-medium text-accent group-hover:underline">
                        {t.programLink}
                        <ArrowRight className="size-4" aria-hidden />
                      </span>
                    </Link>
                  );
                })}
              </div>
            )}
            <p className="mt-5 max-w-2xl text-micro leading-relaxed text-muted">{t.programsNote}</p>
          </div>
        </section>

        <section id="packages" className="px-5 pb-16 sm:px-10 sm:pb-20">
          <div className={wrap}>
            <Heading title={t.packagesHeading} body={t.packagesBody} />
            <Packages state={catalog.products} t={t} onRetry={onRetryCatalog} />
            <Link href="/catalog" className="mt-6 inline-flex min-h-10 items-center gap-2 text-caption font-medium text-accent hover:underline">
              {t.packagesViewAll}
              <ArrowRight className="size-4" aria-hidden />
            </Link>
          </div>
        </section>

        <section className="px-5 pb-16 sm:px-10 sm:pb-20">
          <div className={cn(wrap, "grid items-center gap-9 rounded-rail bg-sidebar p-6 text-sidebar-foreground sm:p-10 md:grid-cols-workspace-band md:gap-11 md:p-12")}>
            <div className="min-w-0">
              <h2 className="font-display text-landing-h2 text-balance">{t.workspaceHeading}</h2>
              <p className="mt-5 max-w-sm text-small leading-relaxed text-sidebar-muted">{t.workspaceBody}</p>
              <Link href="/register" className="mt-6 inline-flex min-h-10 items-center gap-2 text-caption font-medium text-sidebar-accent hover:underline">
                {t.workspaceCta}
                <ArrowRight className="size-4" aria-hidden />
              </Link>
            </div>
            <div role="img" aria-label={t.workspaceLabel} className="min-w-0 overflow-hidden rounded-tile border border-sidebar-border bg-background text-foreground shadow-menu">
              <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3 text-micro text-muted">
                <span className="font-medium text-foreground">{t.workspaceBrand}</span>
                <span>{t.workspaceMenu[0]}</span>
              </div>
              <div className="p-5">
                <p className="font-display text-heading text-foreground">{t.workspaceGreeting}</p>
                <p className="mt-1 text-micro text-muted">{t.workspaceGreetingBody}</p>
                <div className="mt-5 grid grid-cols-3 gap-2.5">
                  {t.workspaceStats.map((label) => (
                    <div key={label} className="rounded-control border border-border bg-surface p-3">
                      <p className="font-display text-metric text-muted">—</p>
                      <p className="mt-1 text-nano leading-snug text-muted">{label}</p>
                    </div>
                  ))}
                </div>
                <div className="mt-4 rounded-tile bg-peach p-5">
                  <p className="text-label font-medium text-foreground">{t.workspaceContinue}</p>
                  <p className="mt-1 max-w-48 text-nano text-foreground">{t.workspaceContinueBody}</p>
                </div>
                <div className="mt-4 flex gap-4 text-nano text-muted">
                  {t.workspaceMenu.map((item, i) => (
                    <span key={item} className={cn(i === 0 && "font-semibold text-accent")}>
                      {item}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </section>

        <section id="cara-kerja" className="scroll-mt-24 px-5 pb-16 sm:px-10 sm:pb-20">
          <div className={wrap}>
            <Heading title={t.stepsHeading} />
            <ol className="grid gap-7 md:grid-cols-3 md:gap-9">
              {t.steps.map((step, i) => (
                <li key={step.title} className="border-t border-border pt-5">
                  <span className="mb-5 block font-display text-small text-accent">{String(i + 1).padStart(2, "0")}</span>
                  <h3 className="font-display text-landing-h3 text-foreground">{step.title}</h3>
                  <p className="mt-3 max-w-sm text-caption leading-relaxed text-muted">{step.body}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="faq" className="scroll-mt-24 px-5 pb-16 sm:px-10 sm:pb-20">
          <div className={cn(wrap, "grid gap-8 md:grid-cols-faq md:gap-16")}>
            <h2 className="font-display text-landing-h2 text-balance text-foreground">{t.faqHeading}</h2>
            <div>
              {t.faq.map((item) => (
                <details key={item.q} className="group border-t border-border last:border-b">
                  <summary className="flex min-h-12 cursor-pointer summary-plain items-center justify-between gap-5 py-5 text-small font-medium text-foreground outline-none focus-visible:ring-2 focus-visible:ring-focus">
                    {item.q}
                    <Plus className="size-4 shrink-0 text-muted transition-transform group-open:rotate-45" aria-hidden />
                  </summary>
                  <p className="max-w-xl pb-6 text-caption leading-loose text-muted">{item.a}</p>
                </details>
              ))}
            </div>
          </div>
        </section>

        <section className="px-5 pb-16 sm:px-10 sm:pb-20">
          <div className={cn(wrap, "rounded-rail bg-peach px-6 py-12 text-center sm:py-14")}>
            <h2 className="font-display text-landing-h2 mx-auto max-w-xl text-balance text-foreground">{t.ctaHeading}</h2>
            <p className="mx-auto mt-5 mb-7 max-w-lg text-small text-foreground">{t.ctaBody}</p>
            <div className="mx-auto grid max-w-md grid-cols-1 gap-3 sm:grid-cols-2">
              <LinkButton href="/#programs" variant="primary" size="lg" className="h-auto min-h-12 min-w-0">
                {home?.program_cta ?? t.fallbackProgramCta}
                <ArrowRight className="size-4 shrink-0" aria-hidden />
              </LinkButton>
              <LinkButton href="/catalog" variant="primary" size="lg" className="h-auto min-h-12 min-w-0">
                {home?.practice_cta ?? t.fallbackPracticeCta}
                <ArrowRight className="size-4 shrink-0" aria-hidden />
              </LinkButton>
            </div>
          </div>
        </section>
      </main>
    </>
  );
}

function Packages({ state, t, onRetry }: { state: Remote<PublicProduct[]>; t: LandingDict; onRetry?: () => void }) {
  if (state.status === "loading") {
    return (
      <div role="status" aria-label={t.packagesLoading} className="border-t border-border">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-4 border-b border-border py-6">
            <div className="size-12 rounded-tile bg-surface-secondary motion-safe:animate-pulse" />
            <div className="h-10 flex-1 rounded-control bg-surface-secondary motion-safe:animate-pulse" />
          </div>
        ))}
      </div>
    );
  }
  if (state.status === "error") {
    return (
      <div role="alert" className="rounded-card border border-border bg-surface p-6">
        <p className="text-small text-foreground">{t.packagesError}</p>
        {onRetry && (
          <Button variant="secondary" size="sm" className="mt-4" onPress={onRetry}>
            {t.packagesRetry}
          </Button>
        )}
      </div>
    );
  }
  if (state.data.length === 0) return <p className="rounded-card border border-border bg-surface p-6 text-small text-muted">{t.packagesEmpty}</p>;

  return (
    <ul className="border-t border-border">
      {state.data.map((product) => {
        const detail =
          product.type === "kecermatan" && product.included_attempts != null
            ? t.packageAttempts(product.included_attempts)
            : product.tryouts.length > 0
              ? t.packageTryouts(product.tryouts.length)
              : null;
        return (
          <li key={product.id}>
            <Link
              href={`/catalog/${product.id}`}
              className="group grid grid-cols-package-row items-center gap-x-4 gap-y-1 border-b border-border px-1 py-5 sm:grid-cols-package-row-wide sm:gap-x-5"
            >
              <span className={cn("row-span-3 flex size-11 items-center justify-center rounded-tile text-accent sm:row-span-1 sm:size-12", product.type === "kecermatan" ? "bg-accent-soft" : "bg-peach")}>
                {product.type === "kecermatan" ? <Star className="size-5" aria-hidden /> : <ShieldCheck className="size-5" aria-hidden />}
              </span>
              <span className="min-w-0">
                <span className="block text-small font-medium break-words text-foreground">{product.title}</span>
                <span className="mt-1 block text-micro text-muted">
                  {[product.program_name, t.packageTypes[product.type], detail].filter(Boolean).join(" · ")}
                </span>
              </span>
              <span className="flex items-baseline gap-2 sm:justify-end">
                <span className="font-display text-title text-foreground">{formatIDR(product.price)}</span>
                {product.original_price && <span className="text-micro text-muted line-through">{formatIDR(product.original_price)}</span>}
              </span>
              <span className="flex items-center gap-1.5 text-caption font-medium text-accent group-hover:underline sm:justify-end">
                {t.packagesDetail}
                <ArrowRight className="size-4" aria-hidden />
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

function Footer({ t }: { t: LandingDict }) {
  const links = [
    ["/#programs", t.footerLinks.programs],
    ["/catalog", t.footerLinks.catalog],
    ["/#cara-kerja", t.footerLinks.howItWorks],
    ["/#faq", t.footerLinks.faq],
    ["/login", t.footerLinks.login],
    ["/register", t.footerLinks.register],
  ] as const;
  return (
    <footer className="px-5 pt-6 pb-8 sm:px-10">
      <div className={wrap}>
        <div className="flex flex-col justify-between gap-6 pb-8 md:flex-row">
          <div>
            <Logo plate size="md" />
            <p className="mt-4 text-label text-muted">{t.footerTagline}</p>
          </div>
          <nav aria-label="Footer" className="flex flex-wrap content-start gap-x-8 gap-y-3 text-caption text-muted">
            {links.map(([href, label]) => (
              <Link key={href} href={href} className="hover:text-foreground">
                {label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex flex-col justify-between gap-1 border-t border-border pt-5 text-micro text-muted sm:flex-row">
          <span>{t.footerRights}</span>
          <span>{t.footerPoweredBy}</span>
        </div>
      </div>
    </footer>
  );
}

export function WebsiteView(props: WebsiteViewProps) {
  const t = landingDict(props.locale);
  return (
    <div className="min-h-dvh bg-background">
      <a href="#main" className="sr-only rounded-control bg-surface px-4 py-3 text-caption focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50">
        {t.skip}
      </a>
      {props.view === "home" && <HomeView {...props} t={t} />}
      <Footer t={t} />
    </div>
  );
}
