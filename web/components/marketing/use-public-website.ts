"use client";

import { useCallback, useEffect, useState } from "react";
import type { Remote, WebsiteLocaleContent } from "@/components/marketing/website-view";
import { visibleSlots, type ProgramVisibility } from "@/components/marketing/hero-exam-preview";
import { api, type PublicProduct } from "@/lib/api/client";

type ContentState = Remote<{ id: WebsiteLocaleContent; en: WebsiteLocaleContent }>;

let publishedRequest: ReturnType<typeof api.website.published> | null = null;
let publishedAt = 0;

/** The page hook and the nav mount together and share one request; the short window keeps later navigations fresh. */
function fetchPublished(fresh = false) {
  if (fresh || !publishedRequest || Date.now() - publishedAt > 2000) {
    publishedAt = Date.now();
    publishedRequest = api.website.published();
    publishedRequest.catch(() => {
      publishedRequest = null;
    });
  }
  return publishedRequest;
}

/** Published website content and catalog products for public pages; they load and fail independently. */
export function usePublicWebsite() {
  const [website, setWebsite] = useState<ContentState>({ status: "loading" });
  const [visibility, setVisibility] = useState<ProgramVisibility | null>(null);
  const [products, setProducts] = useState<Remote<PublicProduct[]>>({ status: "loading" });

  // WEBSITE_NOT_PUBLISHED and network errors both land on the static fallback; public pages never read a draft.
  const loadWebsite = useCallback((fresh = false) => {
    return fetchPublished(fresh)
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
      void loadWebsite(true);
    },
    retryProducts: () => {
      setProducts({ status: "loading" });
      void loadProducts();
    },
  };
}

/** Whether the nav links to programs: unknown until the shared request settles, then false only when every slot is hidden. */
export function useProgramsLinked() {
  const [linked, setLinked] = useState(false);
  useEffect(() => {
    fetchPublished()
      .then((page) => setLinked(visibleSlots(page.content.program_visibility).length > 0))
      .catch(() => setLinked(true));
  }, []);
  return linked;
}
