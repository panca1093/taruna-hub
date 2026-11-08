"use client";

import { useCallback, useEffect, useState } from "react";
import type { Remote, WebsiteLocaleContent } from "@/components/marketing/website-view";
import { visibleSlots, type ProgramVisibility } from "@/components/marketing/hero-exam-preview";
import { api, type PublicProduct } from "@/lib/api/client";

type ContentState = Remote<{ id: WebsiteLocaleContent; en: WebsiteLocaleContent }>;

/** Published website content and catalog products for public pages; they load and fail independently. */
export function usePublicWebsite() {
  const [website, setWebsite] = useState<ContentState>({ status: "loading" });
  const [visibility, setVisibility] = useState<ProgramVisibility | null>(null);
  const [products, setProducts] = useState<Remote<PublicProduct[]>>({ status: "loading" });

  // WEBSITE_NOT_PUBLISHED and network errors both land on the static fallback; public pages never read a draft.
  const loadWebsite = useCallback(() => {
    return api.website
      .published()
      .then((page) => {
        setVisibility(page.content.program_visibility);
        setWebsite({ status: "ready", data: page.content.locales });
      })
      .catch(() => setWebsite({ status: "error" }));
  }, []);

  const loadProducts = useCallback(() => {
    return api.catalog
      .products({ limit: 6 })
      .then((page) => setProducts({ status: "ready", data: page.products }))
      .catch(() => setProducts({ status: "error" }));
  }, []);

  useEffect(() => {
    void loadWebsite();
    void loadProducts();
  }, [loadWebsite, loadProducts]);

  return {
    website,
    visibility,
    products,
    retryWebsite: () => {
      setWebsite({ status: "loading" });
      void loadWebsite();
    },
    retryProducts: () => {
      setProducts({ status: "loading" });
      void loadProducts();
    },
  };
}

/** Whether the nav should link to the programs section; stays true until the published snapshot says every slot is hidden. */
export function useProgramsLinked() {
  const [linked, setLinked] = useState(true);
  useEffect(() => {
    api.website
      .published()
      .then((page) => setLinked(visibleSlots(page.content.program_visibility).length > 0))
      .catch(() => setLinked(true));
  }, []);
  return linked;
}
