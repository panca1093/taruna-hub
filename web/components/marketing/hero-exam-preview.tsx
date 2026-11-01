"use client";

import { useId, useRef, useState, type KeyboardEvent } from "react";
import Link from "next/link";
import { ArrowRight, BookOpen, Circle, Diamond, ShieldCheck, Square, Star, Timer, Triangle } from "lucide-react";
import { CountdownRing } from "../countdown-ring";
import { cn } from "@/lib/cn";
import type { LandingDict } from "@/lib/i18n/dict/landing";

export const programSlots = ["tni", "polri", "kedinasan"] as const;
export type ProgramSlot = (typeof programSlots)[number];

type StageTab = "program" | "practice";
type PracticeKind = "tryout" | "kecermatan";

const stageTabs: StageTab[] = ["program", "practice"];
const referenceIcons = [Circle, Triangle, Square, Diamond, Star];
const choiceLabels = ["A", "B", "C", "D", "E"];

function optionClass(selected: boolean) {
  return cn(
    "flex min-h-11 items-center gap-2.5 rounded-control border px-3 py-2.5 text-left text-caption transition-colors",
    selected ? "border-accent bg-accent-soft text-accent-soft-foreground" : "border-border text-foreground hover:border-accent",
  );
}

function Badge({ selected, children }: { selected: boolean; children: React.ReactNode }) {
  return (
    <span
      className={cn(
        "flex size-5 shrink-0 items-center justify-center rounded-full border text-micro",
        selected ? "border-accent bg-accent text-accent-foreground" : "border-border text-muted",
      )}
    >
      {children}
    </span>
  );
}

/** Labelled illustration only: choices highlight but nothing is graded and no answer key exists here. */
export function HeroExamPreview({ t, programTitles }: { t: LandingDict; programTitles: Record<ProgramSlot, string> }) {
  const baseId = useId();
  const tabRefs = useRef<Record<StageTab, HTMLButtonElement | null>>({ program: null, practice: null });
  const [tab, setTab] = useState<StageTab>("program");
  const [slot, setSlot] = useState<ProgramSlot>("tni");
  const [kind, setKind] = useState<PracticeKind>("tryout");
  const [choice, setChoice] = useState<number | null>(null);

  const tabLabels: Record<StageTab, string> = { program: t.stageTabProgram, practice: t.stageTabPractice };
  const tabId = (name: StageTab) => `${baseId}-tab-${name}`;
  const panelId = (name: StageTab) => `${baseId}-panel-${name}`;

  function onTabKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    const index = stageTabs.indexOf(tab);
    let next: number;
    if (event.key === "ArrowRight") next = (index + 1) % stageTabs.length;
    else if (event.key === "ArrowLeft") next = (index - 1 + stageTabs.length) % stageTabs.length;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = stageTabs.length - 1;
    else return;
    event.preventDefault();
    setTab(stageTabs[next]);
    tabRefs.current[stageTabs[next]]?.focus();
  }

  function switchKind(next: PracticeKind) {
    setKind(next);
    setChoice(null);
  }

  return (
    <div className="animate-stage-in relative min-w-0 rounded-rail bg-peach p-4 sm:p-7">
      <div aria-hidden className="pointer-events-none absolute inset-3 rounded-hero border border-rose/25" />
      <p className="relative mb-4 flex items-center justify-between gap-3 text-micro text-foreground sm:mb-5">
        {t.stageNote}
        <ShieldCheck className="size-4" aria-hidden />
      </p>

      <div className="relative min-h-96 overflow-hidden rounded-card border border-border bg-surface shadow-soft">
        <div role="tablist" aria-label={t.stageTabsLabel} className="flex gap-6 border-b border-border px-5 pt-3">
          {stageTabs.map((name) => (
            <button
              key={name}
              ref={(node) => {
                tabRefs.current[name] = node;
              }}
              id={tabId(name)}
              role="tab"
              type="button"
              aria-selected={tab === name}
              aria-controls={panelId(name)}
              tabIndex={tab === name ? 0 : -1}
              onClick={() => setTab(name)}
              onKeyDown={onTabKeyDown}
              className={cn(
                "-mb-px border-b-2 pt-2 pb-3.5 text-label outline-none focus-visible:ring-2 focus-visible:ring-focus sm:text-caption",
                tab === name ? "border-accent font-medium text-accent" : "border-transparent text-muted",
              )}
            >
              {tabLabels[name]}
            </button>
          ))}
        </div>

        {tab === "program" ? (
          <div id={panelId("program")} role="tabpanel" aria-labelledby={tabId("program")} tabIndex={0} className="p-5 outline-none sm:p-6">
            <div className="mb-5 flex items-center justify-between gap-4">
              <div className="min-w-0">
                <p className="text-label font-medium text-foreground">{t.stageProgramTitle}</p>
                <p className="mt-1 text-micro text-muted">{t.stageProgramBody}</p>
              </div>
              <BookOpen className="size-5 shrink-0 text-muted" aria-hidden />
            </div>
            <div className="grid gap-2.5">
              {programSlots.map((name) => (
                <button
                  key={name}
                  type="button"
                  aria-pressed={slot === name}
                  onClick={() => setSlot(name)}
                  className={cn(
                    "flex items-center gap-3 rounded-control border p-3 text-left transition-colors",
                    slot === name ? "border-accent bg-accent-soft" : "border-border hover:border-accent",
                  )}
                >
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-chip bg-surface text-accent">
                    {name === "kedinasan" ? <BookOpen className="size-4" aria-hidden /> : <ShieldCheck className="size-4" aria-hidden />}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-label font-medium break-words text-foreground">{programTitles[name]}</span>
                    <span className="mt-0.5 block text-micro text-muted">{t.stageProgramRoute}</span>
                  </span>
                  <ArrowRight className="size-4 shrink-0 text-accent" aria-hidden />
                </button>
              ))}
            </div>
            <div className="mt-5 flex items-center justify-between gap-3">
              <p className="max-w-40 text-nano text-muted">{t.stageProgramFoot}</p>
              <Link href="/#programs" className="inline-flex min-h-9 items-center rounded-control bg-accent px-3 text-micro font-medium text-accent-foreground">
                {t.stageProgramCta}
              </Link>
            </div>
          </div>
        ) : (
          <div id={panelId("practice")} role="tabpanel" aria-labelledby={tabId("practice")} tabIndex={0} className="p-5 outline-none sm:p-6">
            <div className="mb-4 flex gap-1.5" role="group" aria-label={t.stagePracticeSwitchLabel}>
              {(["tryout", "kecermatan"] as const).map((name) => (
                <button
                  key={name}
                  type="button"
                  aria-pressed={kind === name}
                  onClick={() => switchKind(name)}
                  className={cn(
                    "rounded-chip px-2.5 py-1.5 text-micro",
                    kind === name ? "bg-accent-soft font-medium text-accent-soft-foreground" : "text-muted",
                  )}
                >
                  {name === "tryout" ? t.stagePracticeTryout : t.stagePracticeKecermatan}
                </button>
              ))}
            </div>

            {kind === "tryout" ? (
              <>
                <div className="mb-4 flex items-center justify-between gap-4">
                  <p className="text-label font-medium text-foreground">{t.stageTryoutType}</p>
                  <span role="img" aria-label={t.stageTimerLabel}>
                    <CountdownRing value={62} size={39} strokeWidth={3} animate={false}>
                      <Timer className="size-3.5 text-accent" aria-hidden />
                    </CountdownRing>
                  </span>
                </div>
                <p className="mb-4 font-display text-heading-sm text-foreground">{t.stageTryoutQuestion}</p>
                <div className="grid grid-cols-2 gap-2.5">
                  {["15", "20", "25", "30"].map((label, i) => (
                    <button key={label} type="button" aria-pressed={choice === i} onClick={() => setChoice(i)} className={optionClass(choice === i)}>
                      <Badge selected={choice === i}>{choiceLabels[i]}</Badge>
                      {label}
                    </button>
                  ))}
                </div>
              </>
            ) : (
              <>
                <p className="mb-3 text-label text-muted">{t.stageKecermatanBody}</p>
                <div className="mb-4 grid grid-cols-5 gap-1.5" aria-hidden>
                  {referenceIcons.map((_, i) => (
                    <div key={i} className="flex justify-center gap-0.5 rounded-chip border border-border bg-surface-secondary px-1 py-2 text-foreground">
                      {[0, 1, 2].map((n) => {
                        const Icon = referenceIcons[(i + n) % referenceIcons.length];
                        return <Icon key={n} className="size-3" />;
                      })}
                    </div>
                  ))}
                </div>
                <div className="flex gap-2">
                  {referenceIcons.map((Icon, i) => (
                    <button
                      key={choiceLabels[i]}
                      type="button"
                      aria-pressed={choice === i}
                      aria-label={choiceLabels[i]}
                      onClick={() => setChoice(i)}
                      className={cn(optionClass(choice === i), "min-w-10 justify-center px-0")}
                    >
                      <Icon className="size-4" aria-hidden />
                    </button>
                  ))}
                </div>
              </>
            )}

            <div className="mt-5 flex items-center justify-between gap-3">
              <p className="max-w-44 text-nano text-muted">{t.stageIllustrationNote}</p>
              <Link href="/catalog" className="inline-flex min-h-9 items-center rounded-control bg-accent px-3 text-micro font-medium text-accent-foreground">
                {t.stagePracticeCta}
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
