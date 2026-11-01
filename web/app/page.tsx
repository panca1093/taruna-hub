"use client";

import { PublicNav } from "@/components/public-nav";
import { WebsiteView } from "@/components/marketing/website-view";
import { usePublicWebsite } from "@/components/marketing/use-public-website";
import { useLocale } from "@/lib/i18n/locale-context";

export default function HomePage() {
  const { locale } = useLocale();
  const { website, products, retryWebsite, retryProducts } = usePublicWebsite();

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
        onRetryContent={retryWebsite}
        onRetryCatalog={retryProducts}
      />
    </>
  );
}
