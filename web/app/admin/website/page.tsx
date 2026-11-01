"use client";

import { ArrowRight, Eye } from "lucide-react";
import { WebsiteFrame, HistoryPanel, previewHref, useWebsiteEditor } from "@/components/admin/website-editor";
import { Metric, MetricStrip } from "@/components/metric-strip";
import { LinkButton } from "@/components/ui/link-button";
import { programSlots } from "@/components/marketing/hero-exam-preview";
import { useWebsiteT } from "@/lib/i18n/dict/website";

function Overview() {
  const t = useWebsiteT();
  const { workbench: wb, diff } = useWebsiteEditor();
  return (
    <div className="space-y-8">
      {wb.published_revision === 0 && (
        <p role="status" className="rounded-tile border border-border bg-accent-soft p-4 text-caption text-accent-soft-foreground">
          {t.firstUse}
        </p>
      )}
      <MetricStrip>
        <Metric value={wb.published_revision === 0 ? "—" : `r${wb.published_revision}`} label={t.statPublished} />
        <Metric value={String(wb.published_revision === 0 ? 0 : diff.length)} label={t.statChanges} />
        <Metric value={String(programSlots.length)} label={t.statPrograms} />
      </MetricStrip>
      <div className="grid gap-5 md:grid-cols-2">
        <section className="flex flex-col rounded-card border border-border bg-surface p-6">
          <h2 className="font-display text-title text-foreground">{t.homeCardTitle}</h2>
          <p className="mt-2 text-caption text-muted">{t.homeCardBody}</p>
          <div className="mt-5 flex flex-wrap gap-2.5">
            <LinkButton href="/admin/website/home" variant="primary" size="sm">
              {t.editHome} <ArrowRight className="size-4" aria-hidden />
            </LinkButton>
            <LinkButton href={previewHref()} variant="secondary" size="sm">
              <Eye className="size-4" aria-hidden /> {t.preview}
            </LinkButton>
          </div>
        </section>
        <section className="flex flex-col rounded-card border border-border bg-surface p-6">
          <h2 className="font-display text-title text-foreground">{t.programsCardTitle}</h2>
          <p className="mt-2 text-caption text-muted">{t.programsCardBody}</p>
          <div className="mt-5 flex flex-wrap gap-2.5">
            <LinkButton href="/admin/website/programs" variant="primary" size="sm">
              {t.editPrograms} <ArrowRight className="size-4" aria-hidden />
            </LinkButton>
          </div>
        </section>
      </div>
      <div className="grid gap-5 border-t border-border pt-6 text-caption md:grid-cols-2">
        <div>
          <p className="font-medium text-foreground">{t.fixedNoteTitle}</p>
          <p className="mt-1 text-muted">{t.fixedNoteBody}</p>
        </div>
        <div>
          <p className="font-medium text-foreground">{t.productNoteTitle}</p>
          <p className="mt-1 text-muted">{t.productNoteBody}</p>
        </div>
      </div>
      <HistoryPanel />
    </div>
  );
}

export default function WebsiteOverviewPage() {
  const t = useWebsiteT();
  return (
    <WebsiteFrame
      active="overview"
      title={t.overviewTitle}
      description={t.overviewDescription}
      actions={
        <>
          <LinkButton href="/" variant="secondary" size="sm" target="_blank">
            <Eye className="size-4" aria-hidden /> {t.viewPublished}
          </LinkButton>
          <LinkButton href="/admin/website/review" variant="primary" size="sm">
            {t.review}
          </LinkButton>
        </>
      }
    >
      <Overview />
    </WebsiteFrame>
  );
}
