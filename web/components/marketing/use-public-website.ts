"use client";

import { useCallback, useEffect, useState } from "react";
import type { Remote, WebsiteLocaleContent } from "@/components/marketing/website-view";
import { api, type PublicProduct } from "@/lib/api/client";

type ContentState = Remote<{ id: WebsiteLocaleContent; en: WebsiteLocaleContent }>;

/** Published website content and catalog products for public pages; they load and fail independently. */
export function usePublicWebsite() {
  const [website, setWebsite] = useState<ContentState>({ status: "loading" });
  const [products, setProducts] = useState<Remote<PublicProduct[]>>({ status: "loading" });

  // WEBSITE_NOT_PUBLISHED and network errors both land on the static fallback; public pages never read a draft.
  const loadWebsite = useCallback(() => {
    return api.website
      .published()
      .then((page) => setWebsite({ status: "ready", data: page.content.locales }))
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
