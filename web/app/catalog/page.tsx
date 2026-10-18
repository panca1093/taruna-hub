"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button, Card, Chip } from "@heroui/react";
import { PackageSearch } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { PublicNav } from "@/components/public-nav";
import { EmptyState } from "@/components/empty-state";
import { SelectInput, TextInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { useCatalogT } from "@/lib/i18n/dict/catalog";
import { api, type PublicProduct } from "@/lib/api/client";
import { formatIDR } from "@/lib/format-money";
import { cn } from "@/lib/cn";

type TypeFilter = "all" | "individual" | "bundle";

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

  const renderCard = (product: PublicProduct) => (
    <Link key={product.id} href={`/catalog/${product.id}`}>
      <Card className="group h-full gap-0 overflow-hidden p-0 transition-shadow hover:shadow-overlay">
        <div className="relative h-32 w-full bg-gradient-ring">
          <div className="absolute top-3 left-3 flex gap-1.5">
            <Chip size="sm" className="bg-white/20 text-white backdrop-blur-sm">
              {product.program_code}
            </Chip>
            <Chip size="sm" className="bg-white/20 text-white backdrop-blur-sm">
              {product.type === "bundle" ? t.bundleSize(product.tryouts.length) : t.typeIndividual}
            </Chip>
            {product.discount_percent !== null && (
              <Chip size="sm" className="bg-white/20 text-white backdrop-blur-sm">
                {t.discountChip(product.discount_percent)}
              </Chip>
            )}
          </div>
        </div>
        <div className="space-y-2.5 p-4">
          <p className="line-clamp-1 text-sm font-semibold text-foreground">{product.title}</p>
          <p className="line-clamp-2 text-xs text-muted">{product.description}</p>
          <div className="flex items-baseline gap-2 pt-1">
            <span className="font-display text-base font-semibold text-foreground">{formatIDR(product.price)}</span>
            {product.original_price && (
              <span className="text-xs text-muted line-through">{formatIDR(product.original_price)}</span>
            )}
          </div>
        </div>
      </Card>
    </Link>
  );

  const tabs: { key: TypeFilter; label: string }[] = [
    { key: "all", label: t.filterAllType },
    { key: "individual", label: t.typeIndividual },
    { key: "bundle", label: t.typeBundle },
  ];

  const body = (
    <>
      <div className="flex flex-col gap-3 border-b border-border sm:flex-row sm:items-center sm:justify-between sm:gap-4">
        <div className="flex gap-6">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setType(tab.key)}
              className={cn(
                "relative flex-1 pb-3 text-sm font-medium whitespace-nowrap transition-colors sm:flex-none",
                type === tab.key ? "text-foreground" : "text-muted hover:text-foreground",
              )}
            >
              {tab.label}
              {type === tab.key && (
                <span className="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-gradient-ring" aria-hidden />
              )}
            </button>
          ))}
        </div>
        <div className="flex flex-col gap-3 pb-3 sm:flex-row sm:items-center sm:pb-2.5">
          <div className="sm:w-52">
            <TextInput
              aria-label={t.searchLabel}
              placeholder={t.searchPlaceholder}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="sm:w-52">
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
        <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="h-64 animate-pulse rounded-xl border border-border bg-surface-secondary" />
          ))}
        </div>
      ) : unknownProgram || rows.length === 0 ? (
        <EmptyState className="mt-10" icon={PackageSearch} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{rows.map(renderCard)}</div>
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
        <p className="text-sm font-medium text-muted">{t.publicEyebrow}</p>
        <h1 className="font-display mt-1 text-3xl font-semibold tracking-tight text-foreground">{t.publicHeading}</h1>
        <p className="mt-2 max-w-xl text-sm text-muted">{t.description}</p>
        <div className="mt-8">{body}</div>
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
