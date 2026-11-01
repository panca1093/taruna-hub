"use client";

import { use } from "react";
import { notFound } from "next/navigation";
import { WebsiteFrame, ContentField, LocaleSwitch, SaveBar, SaveFeedback, useWebsiteEditor } from "@/components/admin/website-editor";
import { ProgramDetail } from "@/components/marketing/website-view";
import { programSlots, type ProgramSlot } from "@/components/marketing/hero-exam-preview";
import { useWebsiteT } from "@/lib/i18n/dict/website";

function ProgramEditor({ slot }: { slot: ProgramSlot }) {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  return (
    <div className="space-y-5">
      <LocaleSwitch />
      <div className="grid items-start gap-6 lg:grid-cols-2">
        <section className="space-y-5 rounded-card border border-border bg-surface p-6">
          <ContentField scope={slot} fieldKey="title" />
          <ContentField scope={slot} fieldKey="summary" multiline rows={4} />
          <ContentField scope={slot} fieldKey="schedule" help={t.fieldHelp.optional} />
          <ContentField scope={slot} fieldKey="location" help={t.fieldHelp.optional} />
          <ContentField scope={slot} fieldKey="fees" help={t.fieldHelp.fees} />
          <ContentField scope={slot} fieldKey="contact" help={t.fieldHelp.contact} />
          <SaveFeedback />
          <SaveBar slot={slot} />
        </section>
        <section className="rounded-card border border-border bg-surface p-6 lg:sticky lg:top-6">
          <h2 className="mb-2 font-display text-title text-foreground">{t.previewTitle}</h2>
          <div inert>
            <ProgramDetail program={ed.local.locales[ed.contentLocale].programs[slot]} locale={ed.contentLocale} showBack={false} />
          </div>
          <p className="mt-2 text-label text-muted">{t.previewNote}</p>
        </section>
      </div>
    </div>
  );
}

export default function WebsiteProgramPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = use(params);
  const t = useWebsiteT();
  if (!(programSlots as readonly string[]).includes(slug)) notFound();
  const slot = slug as ProgramSlot;
  return (
    <WebsiteFrame
      active="programs"
      title={t.programEditTitle(t.programNames[slot])}
      description={t.programEditDescription}
      breadcrumb={[{ label: t.pageTitle, href: "/admin/website" }, { label: t.programsTitle, href: "/admin/website/programs" }, { label: t.programNames[slot] }]}
    >
      <ProgramEditor slot={slot} />
    </WebsiteFrame>
  );
}
