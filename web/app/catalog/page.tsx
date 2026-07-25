"use client";

import { Suspense, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Card, Chip } from "@heroui/react";
import { PackageSearch } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { PublicNav } from "@/components/public-nav";
import { EmptyState } from "@/components/empty-state";
import { SelectInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { useCatalogT } from "@/lib/i18n/dict/catalog";
import { programs, products, formatIDR, getProgram } from "@/lib/mock-data";
import type { Product } from "@/lib/types";
import { cn } from "@/lib/cn";

type Category = "academic" | "psychometric";

function CatalogContent() {
  const params = useSearchParams();
  const [program, setProgram] = useState<string>(params.get("program") ?? "all");
  const [category, setCategory] = useState<Category>(params.get("category") === "psychometric" ? "psychometric" : "academic");
  const { role } = useSession();
  const t = useCatalogT();

  // Computed per render so the program dropdown follows programs configured by admin.
  const programOptions: { label: string; id: string }[] = [
    { id: "all", label: t.filterAll },
    ...programs.map((p) => ({ id: p.code, label: p.code })),
  ];

  const byProgram = useMemo(() => {
    const activeProducts = products.filter((p) => p.isActive);
    if (program === "all") return activeProducts;
    const prog = programs.find((p) => p.code === program);
    return activeProducts.filter((p) => p.programId === prog?.id);
  }, [program]);

  const sectionProducts = byProgram.filter((p) => (category === "psychometric" ? p.type === "kecermatan" : p.type !== "kecermatan"));

  const renderCard = (product: Product) => {
    const prog = getProgram(product.programId);
    return (
      <Link key={product.id} href={`/catalog/${product.id}`}>
        <Card className="group h-full gap-0 overflow-hidden p-0 transition-shadow hover:shadow-overlay">
          <div
            className="relative h-32 w-full"
            style={{ background: `linear-gradient(135deg, ${product.coverFrom}, ${product.coverTo})` }}
          >
            <div className="absolute top-3 left-3 flex gap-1.5">
              {prog && (
                <Chip size="sm" className="bg-white/20 text-white backdrop-blur-sm">
                  {prog.code}
                </Chip>
              )}
              <Chip size="sm" className="bg-white/20 text-white backdrop-blur-sm">
                {product.type === "bundle"
                  ? t.typeBundle(product.tryoutIds.length)
                  : product.type === "kecermatan"
                    ? t.typeKecermatan
                    : t.typeIndividual}
              </Chip>
            </div>
          </div>
          <div className="space-y-2.5 p-4">
            <p className="line-clamp-1 text-sm font-semibold text-foreground">{product.title}</p>
            <p className="line-clamp-2 text-xs text-muted">{product.description}</p>
            <div className="flex items-baseline gap-2 pt-1">
              <span className="font-display text-base font-semibold text-foreground">
                {formatIDR(product.price)}
              </span>
              <span className="text-xs text-muted line-through">{formatIDR(product.originalPrice)}</span>
            </div>
          </div>
        </Card>
      </Link>
    );
  };

  const tabs: { key: Category; label: string }[] = [
    { key: "academic", label: t.sectionAcademic },
    { key: "psychometric", label: t.sectionPsychometric },
  ];

  const body = (
    <>
      <div className="flex flex-col gap-3 border-b border-border sm:flex-row sm:items-center sm:justify-between sm:gap-4">
        <div className="flex gap-6">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setCategory(tab.key)}
              className={cn(
                "relative flex-1 pb-3 text-sm font-medium whitespace-nowrap transition-colors sm:flex-none",
                category === tab.key ? "text-foreground" : "text-muted hover:text-foreground",
              )}
            >
              {tab.label}
              {category === tab.key && (
                <span className="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-gradient-ring" aria-hidden />
              )}
            </button>
          ))}
        </div>
        <div className="pb-3 sm:w-48 sm:pb-2.5">
          <SelectInput
            options={programOptions}
            selectedKey={program}
            onSelectionChange={(k) => setProgram(k ?? "all")}
          />
        </div>
      </div>

      {sectionProducts.length === 0 ? (
        <EmptyState className="mt-10" icon={PackageSearch} title={t.emptyTitle} description={t.emptyDescription} />
      ) : (
        <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{sectionProducts.map(renderCard)}</div>
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
