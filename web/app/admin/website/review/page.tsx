"use client";

import { useState } from "react";
import { Button } from "@heroui/react";
import { Check, Eye } from "lucide-react";
import {
  Banner,
  HistoryPanel,
  SaveFeedback,
  WebsiteFrame,
  fieldPath,
  previewHref,
  useWebsiteEditor,
  type DiffEntry,
} from "@/components/admin/website-editor";
import { Icon } from "@/components/icon";
import { LinkButton } from "@/components/ui/link-button";
import { useWebsiteT } from "@/lib/i18n/dict/website";

function DiffRow({ entry }: { entry: DiffEntry }) {
  const t = useWebsiteT();
  const group = entry.scope === "home" ? t.homeGroup : t.programGroup(t.programNames[entry.scope]);
  const show = (v: string | null) => (v === null ? t.beforeNone : v.trim() === "" ? t.emptyValue : v);
  return (
    <li className="border-b border-border py-5 last:border-b-0">
      <p className="text-caption font-medium text-foreground">
        {group} · {t.localeNames[entry.locale]} · {t.fields[entry.key]}
      </p>
      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <div className="min-w-0 p-3">
          <p className="text-micro text-muted">{t.before}</p>
          <p className="mt-1.5 text-caption break-words whitespace-pre-line text-muted">{show(entry.before)}</p>
        </div>
        <div className="min-w-0 rounded-tile bg-accent-soft p-3 text-accent-soft-foreground">
          <p className="text-micro">{t.after}</p>
          <p className="mt-1.5 text-caption break-words whitespace-pre-line">{show(entry.after)}</p>
        </div>
      </div>
    </li>
  );
}

function Review() {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  const { workbench: wb, diff, dirty, publishState: pub } = ed;
  const pairKey = `${wb.draft_revision}:${wb.published_revision}`;
  const [ackedFor, setAckedFor] = useState<string | null>(null);
  const acked = ackedFor === pairKey;
  const inFlight = pub.status === "publishing" || pub.status === "checking" || ed.save.status === "saving" || ed.save.status === "checking";
  const canPublish = !dirty && diff.length > 0 && acked && !inFlight;

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      {pub.status === "success" && (
        <Banner tone="success" title={t.publishSuccessTitle}>
          <p>{t.publishSuccessBody(pub.revision)}</p>
          {pub.changed === false && <p>{t.publishNoop}</p>}
        </Banner>
      )}
      {pub.status === "failed" && (
        <Banner tone="danger" title={t.publishFailedTitle}>
          <p>{pub.message ?? t.publishFailedBody}</p>
          {pub.requestId && <p className="font-mono text-label">{pub.requestId}</p>}
        </Banner>
      )}
      {pub.status === "checking" && <Banner tone="warning" title={t.publishCheckingTitle}>{t.publishCheckingBody}</Banner>}
      {pub.status === "conflict" && <Banner tone="warning" title={t.publishConflictTitle}>{t.publishConflictBody}</Banner>}
      {dirty && (
        <Banner tone="warning" title={t.saveFirst}>
          <SaveFeedback />
          <Button size="sm" variant="secondary" isDisabled={inFlight || Object.keys(ed.clientErrors).length > 0} onPress={() => void ed.doSave()}>
            {t.save}
          </Button>
        </Banner>
      )}

      {diff.length === 0 ? (
        <section className="rounded-card border border-border bg-surface p-6">
          <h2 className="font-display text-title text-foreground">{t.noDiffTitle}</h2>
          <p className="mt-1 text-caption text-muted">{t.noDiffBody}</p>
        </section>
      ) : (
        <section aria-labelledby="diff-heading" className="rounded-card border border-border bg-surface p-6">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 id="diff-heading" className="font-display text-title text-foreground">{t.changeCount(diff.length)}</h2>
            <span className="rounded-chip bg-accent-soft px-2.5 py-1 text-label text-accent-soft-foreground">
              {t.revisionPair(wb.draft_revision, wb.published_revision)}
            </span>
          </div>
          <ul className="mt-2">
            {diff.map((entry) => (
              <DiffRow key={fieldPath(entry)} entry={entry} />
            ))}
          </ul>
        </section>
      )}

      <div className="space-y-4 rounded-card border border-border bg-surface p-6">
        <label className="flex min-h-11 cursor-pointer items-center gap-3 text-caption text-foreground">
          <input
            type="checkbox"
            className="size-4 accent-accent"
            checked={acked}
            disabled={diff.length === 0 || dirty}
            onChange={(e) => setAckedFor(e.target.checked ? pairKey : null)}
          />
          {t.ack}
        </label>
        <div className="flex flex-wrap gap-2.5">
          <LinkButton href={previewHref()} variant="secondary">
            <Icon icon={Eye} size="sm" /> {t.preview}
          </LinkButton>
          <Button variant="primary" isDisabled={!canPublish} onPress={() => void ed.doPublish()}>
            <Icon icon={Check} size="sm" /> {pub.status === "publishing" ? t.publishing : t.publish}
          </Button>
        </div>
      </div>

      <HistoryPanel />
    </div>
  );
}

export default function WebsiteReviewPage() {
  const t = useWebsiteT();
  return (
    <WebsiteFrame active="review" title={t.reviewTitle} description={t.reviewDescription}>
      <Review />
    </WebsiteFrame>
  );
}
