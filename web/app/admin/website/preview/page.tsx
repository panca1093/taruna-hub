"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { notFound, useSearchParams } from "next/navigation";
import { ContentLocaleButtons, WebsiteGate, useWebsiteEditor } from "@/components/admin/website-editor";
import { LinkButton } from "@/components/ui/link-button";
import { WebsiteView, type Remote, type WebsiteViewName } from "@/components/marketing/website-view";
import { programSlots, type ProgramSlot } from "@/components/marketing/hero-exam-preview";
import { api, type PublicProduct } from "@/lib/api/client";
import { useWebsiteT } from "@/lib/i18n/dict/website";

const hrefs = { home: "/admin/website/preview", program: (slot: ProgramSlot) => `/admin/website/preview?program=${slot}` };

function Preview({ slot }: { slot: ProgramSlot | null }) {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  const [products, setProducts] = useState<Remote<PublicProduct[]>>({ status: "loading" });

  const loadProducts = useCallback(
    () =>
      api.catalog
        .products({ limit: 6 })
        .then((page) => setProducts({ status: "ready", data: page.products }))
        .catch(() => setProducts({ status: "error" })),
    [],
  );
  useEffect(() => {
    void loadProducts();
  }, [loadProducts]);

  const view: WebsiteViewName = slot ?? "home";
  return (
    <WebsiteView
      mode="draft"
      view={view}
      locale={ed.contentLocale}
      content={ed.workbench.draft.locales[ed.contentLocale]}
      contentState="ready"
      catalog={{ products }}
      hrefs={hrefs}
      onRetryCatalog={() => {
        setProducts({ status: "loading" });
        void loadProducts();
      }}
      draftAction={
        <div className="flex flex-wrap items-center gap-2">
          <ContentLocaleButtons />
          <LinkButton href={slot ? `/admin/website/programs/${slot}` : "/admin/website/home"} variant="primary" size="sm">
            {slot ? t.previewReturnProgram : t.previewReturn}
          </LinkButton>
        </div>
      }
    />
  );
}

function PreviewRoute() {
  const param = useSearchParams().get("program");
  if (param !== null && !(programSlots as readonly string[]).includes(param)) notFound();
  return (
    <WebsiteGate>
      <Preview slot={param as ProgramSlot | null} />
    </WebsiteGate>
  );
}

export default function WebsitePreviewPage() {
  return (
    <Suspense>
      <PreviewRoute />
    </Suspense>
  );
}
