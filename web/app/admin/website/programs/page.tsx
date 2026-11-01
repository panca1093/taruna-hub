"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { WebsiteFrame, LocaleSwitch, useWebsiteEditor } from "@/components/admin/website-editor";
import { StatusChip } from "@/components/status-badge";
import { programSlots } from "@/components/marketing/hero-exam-preview";
import { useWebsiteT } from "@/lib/i18n/dict/website";

const optionalKeys = ["schedule", "location", "fees", "contact"] as const;

function Slots() {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  const locale = ed.contentLocale;
  return (
    <div className="space-y-5">
      <LocaleSwitch />
      <ul className="grid gap-5 md:grid-cols-3">
        {programSlots.map((slot) => {
          const program = ed.local.locales[locale].programs[slot];
          const filled = optionalKeys.filter((k) => program[k].trim() !== "").length;
          return (
            <li key={slot} className="flex flex-col rounded-card border border-border bg-surface p-6">
              <p className="text-micro text-muted">{t.programNames[slot]}</p>
              <h2 className="mt-1 font-display text-title break-words text-foreground">{program.title}</h2>
              <p className="mt-2 text-label leading-relaxed break-words text-muted">{program.summary}</p>
              <div className="mt-4 flex flex-wrap items-center gap-2 text-label text-muted">
                {filled < optionalKeys.length && <StatusChip tone="warning">{t.programPending}</StatusChip>}
                <span>{t.programFilled(filled, optionalKeys.length)}</span>
              </div>
              <Link href={`/admin/website/programs/${slot}`} className="mt-auto flex min-h-11 items-center gap-2 pt-5 text-caption font-medium text-accent outline-none hover:underline focus-visible:ring-2 focus-visible:ring-focus">
                {t.editProgram}
                <ArrowRight className="size-4" aria-hidden />
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export default function WebsiteProgramsPage() {
  const t = useWebsiteT();
  return (
    <WebsiteFrame active="programs" title={t.programsTitle} description={t.programsDescription}>
      <Slots />
    </WebsiteFrame>
  );
}
