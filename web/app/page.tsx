"use client";

import { useCallback, useEffect, useState } from "react";
import { PublicNav } from "@/components/public-nav";
import { WebsiteView, type Remote, type WebsiteLocaleContent } from "@/components/marketing/website-view";
import { api, type PublicProduct } from "@/lib/api/client";
import { useLocale } from "@/lib/i18n/locale-context";

type ContentState = Remote<{ id: WebsiteLocaleContent; en: WebsiteLocaleContent }>;

export default function HomePage() {
  const { locale } = useLocale();
  const [website, setWebsite] = useState<ContentState>({ status: "loading" });
  const [products, setProducts] = useState<Remote<PublicProduct[]>>({ status: "loading" });

  // WEBSITE_NOT_PUBLISHED and network errors both land on the static fallback; the public route never reads a draft.
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

  return (
    <>
      <PublicNav />
      <WebsiteView
        mode="published"
        view="home"
        locale={locale}
        content={website.status === "ready" ? website.data[locale] : null}
        contentState={website.status === "ready" ? "ready" : website.status === "loading" ? "loading" : "unavailable"}
        catalog={{ products }}
        onRetryContent={() => {
          setWebsite({ status: "loading" });
          void loadWebsite();
        }}
        onRetryCatalog={() => {
          setProducts({ status: "loading" });
          void loadProducts();
        }}
      />
    </>
  );
}
