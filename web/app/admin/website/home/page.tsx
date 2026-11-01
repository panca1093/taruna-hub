"use client";

import { WebsiteFrame, ContentField, LocaleSwitch, SaveBar, SaveFeedback, useWebsiteEditor } from "@/components/admin/website-editor";
import { HeroPreview } from "@/components/marketing/website-view";
import { useWebsiteT } from "@/lib/i18n/dict/website";

function HomeEditor() {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  return (
    <div className="space-y-5">
      <LocaleSwitch />
      <div className="grid items-start gap-6 lg:grid-cols-2">
        <section className="space-y-5 rounded-card border border-border bg-surface p-6">
          <h2 className="font-display text-title text-foreground">{t.heroSection}</h2>
          <ContentField scope="home" fieldKey="hero_title" multiline rows={2} />
          <ContentField scope="home" fieldKey="hero_body" multiline rows={4} />
          <ContentField scope="home" fieldKey="program_cta" />
          <ContentField scope="home" fieldKey="practice_cta" />
          <p className="text-label text-muted">{t.ctaNote}</p>
          <SaveFeedback />
          <SaveBar />
        </section>
        <section className="rounded-card border border-border bg-surface p-6 lg:sticky lg:top-6">
          <h2 className="mb-4 font-display text-title text-foreground">{t.previewTitle}</h2>
          <HeroPreview home={ed.local.locales[ed.contentLocale].home} locale={ed.contentLocale} />
          <p className="mt-4 text-label text-muted">{t.previewNote}</p>
        </section>
      </div>
    </div>
  );
}

export default function WebsiteHomePage() {
  const t = useWebsiteT();
  return (
    <WebsiteFrame active="home" title={t.homeTitle} description={t.homeDescription}>
      <HomeEditor />
    </WebsiteFrame>
  );
}
