"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@heroui/react";
import { ArrowRight, BookOpen, Grid2x2, Layers, PackageSearch, Search } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { PublicNav } from "@/components/public-nav";
import { EmptyState } from "@/components/empty-state";
import { FilterPills } from "@/components/filter-pills";
import { Icon } from "@/components/icon";
import { CardKicker } from "@/components/page-blocks";
import { SelectInput, TextInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { useCatalogT } from "@/lib/i18n/dict/catalog";
import { api, type PublicProduct } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { cn } from "@/lib/cn";

type TypeFilter = "all" | "individual" | "bundle" | "kecermatan";

const PAGE_SIZE = 60;

function CatalogContent() {
  const params = useSearchParams();
  const [programCode, setProgramCode] = useState<string>(params.get("program") ?? "all");
  const [type, setType] = useState<TypeFilter>("all");
  const [search, setSearch] = useState("");
  const [appliedSearch, setAppliedSearch] = useState("");
  const { role } = useSession();
  const t = useCatalogT();

  const [options, setOptions] = useState<PublicProduct[] | null>(null);
  const [rows, setRows] = useState<PublicProduct[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api.catalog
      .products({ limit: 100 })
      .then((page) => {
        if (!cancelled) setOptions(page.products);
      })
      .catch(() => {
        if (!cancelled) setOptions([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => setAppliedSearch(search.trim()), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const programs = (options ?? []).reduce<{ id: string; label: string }[]>((list, product) => {
    if (!list.some((item) => item.id === product.program_code)) {
      list.push({ id: product.program_code, label: `${product.program_code} · ${product.program_name}` });
    }
    return list;
  }, []);

  const programID = options?.find((product) => product.program_code === programCode)?.program_id;
  const unknownProgram = options !== null && programCode !== "all" && programID === undefined;

  useEffect(() => {
    if (options === null || unknownProgram) return;
    let cancelled = false;
    api.catalog
      .products({
        ...(programID ? { program_id: programID } : {}),
        ...(type === "all" ? {} : { type }),
        ...(appliedSearch === "" ? {} : { q: appliedSearch }),
        limit: PAGE_SIZE,
      })
      .then((page) => {
        if (cancelled) return;
        setRows(page.products);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [options, unknownProgram, programID, type, appliedSearch, reloadKey]);

  const retry = () => {
    setRows(null);
    setLoadFailed(false);
    setReloadKey((key) => key + 1);
  };

  const renderCard = (product: PublicProduct) => {
    const kind = product.type === "kecermatan" ? "kecermatan" : product.type === "bundle" ? "bundle" : "individual";
    const symbol = {
      individual: { glyph: BookOpen, tone: "bg-accent-soft text-accent" },
      bundle: { glyph: Layers, tone: "bg-peach text-accent" },
      kecermatan: { glyph: Grid2x2, tone: "bg-success-soft text-success-soft-foreground" },
    }[kind];
    const typeLabel =
      kind === "bundle" ? t.bundleSize(product.tryouts.length) : kind === "kecermatan" ? t.typeKecermatan : t.typeIndividual;
    return (
      <Link
        key={product.id}
        href={`/catalog/${product.id}`}
        className="group flex min-h-70 flex-col rounded-card border border-border bg-surface p-6 outline-none transition-colors hover:border-accent focus-visible:ring-2 focus-visible:ring-focus"
      >
        <span className={cn("mb-6 grid size-13 shrink-0 place-items-center rounded-control", symbol.tone)}>
          <Icon icon={symbol.glyph} size="md" />
        </span>
        <CardKicker className="mb-3">
          {product.program_code} · {typeLabel}
        </CardKicker>
        <h3 className="font-display mb-2.5 line-clamp-2 text-title text-foreground">{product.title}</h3>
        <p className="mb-6 line-clamp-2 text-caption text-muted">{product.description}</p>
        <div className="mt-auto flex flex-wrap items-center justify-between gap-3.5 pt-4">
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
            <span className="font-display text-title text-foreground">{formatIDR(product.price)}</span>
            {product.original_price && (
              <span className="text-label text-muted line-through">{formatIDR(product.original_price)}</span>
            )}
            {product.discount_percent !== null && (
              <span className="rounded-chip bg-success-soft px-2 py-0.5 text-nano text-success-soft-foreground">
                {t.discountChip(product.discount_percent)}
              </span>
            )}
          </div>
          <span className="inline-flex items-center gap-1.5 text-label font-medium text-accent group-hover:underline">
            {t.viewDetail}
            <Icon icon={ArrowRight} size="xs" />
          </span>
        </div>
      </Link>
    );
  };

  const tabs = [
    { id: "all", label: t.filterAllType },
    { id: "individual", label: t.typeIndividual },
    { id: "bundle", label: t.typeBundle },
    { id: "kecermatan", label: t.sectionPsychometric },
  ];

  const body = (
    <>
      <div className="mb-6 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <FilterPills options={tabs} value={type} onChange={(id) => setType(id as TypeFilter)} ariaLabel={t.filterAllType} />
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div className="relative sm:w-56">
            <Icon icon={Search} size="xs" className="pointer-events-none absolute top-1/2 left-3.5 z-10 -translate-y-1/2 text-muted" />
            <TextInput
              aria-label={t.searchLabel}
              placeholder={t.searchPlaceholder}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              inputClassName="pl-9"
            />
          </div>
          <div className="sm:w-56">
            <SelectInput
              options={[{ id: "all", label: t.filterAllProgram }, ...programs]}
              selectedKey={programCode}
              onSelectionChange={(key) => setProgramCode(key ?? "all")}
            />
          </div>
        </div>
      </div>

      {loadFailed ? (
        <EmptyState
          className="mt-10"
          icon={PackageSearch}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" size="sm" onPress={retry}>
              {t.retry}
            </Button>
          }
        />
      ) : rows === null ? (
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="h-70 animate-pulse rounded-card border border-border bg-surface" />
          ))}
        </div>
      ) : unknownProgram || rows.length === 0 ? (
        <EmptyState className="mt-10" icon={PackageSearch} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{rows.map(renderCard)}</div>
      )}
    </>
  );

  if (role === "student") {
    return (
      <AppShell title={t.title} description={t.description} maxWidth="max-w-6xl">
        {body}
      </AppShell>
    );
  }

  return (
    <div className="min-h-dvh bg-background">
      <PublicNav />
      <div className="mx-auto max-w-6xl px-5 py-12">
        <p className="text-small text-muted">{t.publicEyebrow}</p>
        <h1 className="font-display mt-1 text-display-sm md:text-display text-foreground">{t.publicHeading}</h1>
        <p className="mt-2 mb-7 max-w-prose text-small text-muted">{t.description}</p>
        {body}
      </div>
    </div>
  );
}

export default function CatalogPage() {
  return (
    <Suspense fallback={null}>
      <CatalogContent />
    </Suspense>
  );
}
