"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { LatexText } from "@/components/latex";
import { inlineImages } from "@/lib/capture-pdf";

export interface PrintQuestionData {
  type: string;
  content: string;
  options: { label: string }[];
}

export interface PrintSubtestData {
  name: string;
  durationSeconds: number;
  questions: PrintQuestionData[];
}

export interface TryoutPrintLabels {
  subtestMeta: (minutes: number, questions: number) => string;
  pageOf: (page: number, total: number) => string;
  answerLine: string;
  trueLabel: string;
  falseLabel: string;
  imageFailed: string;
}

interface TryoutPrintProps {
  title: string;
  tenantName: string;
  date: string;
  subtests: PrintSubtestData[];
  labels: TryoutPrintLabels;
  /** Called once with the finished A4 page elements, in order. */
  onLaidOut: (pages: HTMLElement[]) => void;
}

// A4 at 96 dpi; the capture runs at 2x, so the PDF stays sharp.
const PAGE_W = 794;
const PAGE_H = 1123;
const PAD_X = 56;
const PAD_TOP = 48;
const FOOTER_H = 72;
const BODY_W = PAGE_W - PAD_X * 2;
const BODY_H = PAGE_H - PAD_TOP - FOOTER_H;
const LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";

type Block =
  | { kind: "title"; title: string }
  | { kind: "subtest"; name: string; minutes: number; count: number }
  | { kind: "question"; number: number; question: PrintQuestionData };

interface PageLayout {
  blocks: { index: number; scale: number }[];
}

/**
 * Flows measured block heights into pages. A `keep` block stays on the page of the block after it; a group whose
 * keep blocks plus the block after them exceed a page goes on its own page with that last block scaled to fit.
 */
export function paginate(heights: number[], keep: boolean[], bodyH: number): PageLayout[] {
  const pages: PageLayout[] = [];
  let current: PageLayout["blocks"] = [];
  let used = 0;
  const flush = () => {
    if (current.length > 0) pages.push({ blocks: current });
    current = [];
    used = 0;
  };
  const place = (i: number) => {
    if (heights[i] > bodyH) {
      flush();
      pages.push({ blocks: [{ index: i, scale: bodyH / heights[i] }] });
      return;
    }
    if (used + heights[i] > bodyH) flush();
    current.push({ index: i, scale: 1 });
    used += heights[i];
  };
  let i = 0;
  while (i < heights.length) {
    let end = i;
    while (keep[end] && end + 1 < heights.length) end++;
    const lead = heights.slice(i, end).reduce((sum, h) => sum + h, 0);
    const total = lead + heights[end];
    if (total <= bodyH) {
      if (used + total > bodyH) flush();
      for (let j = i; j <= end; j++) place(j);
    } else if (!keep[end] && lead < bodyH) {
      flush();
      const blocks = Array.from({ length: end - i }, (_, k) => ({ index: i + k, scale: 1 }));
      pages.push({ blocks: [...blocks, { index: end, scale: (bodyH - lead) / heights[end] }] });
    } else {
      for (let j = i; j <= end; j++) place(j);
    }
    i = end + 1;
  }
  flush();
  return pages;
}

function BlockView({ block, labels }: { block: Block; labels: TryoutPrintLabels }) {
  if (block.kind === "title") {
    return <h1 className="font-display text-display-sm text-foreground pb-6">{block.title}</h1>;
  }
  if (block.kind === "subtest") {
    return (
      <div className="border-b border-border pb-4 pt-2">
        <h2 className="font-display text-heading-sm text-foreground">{block.name}</h2>
        <p className="text-small text-muted">{labels.subtestMeta(block.minutes, block.count)}</p>
      </div>
    );
  }
  const { question } = block;
  const options =
    question.type === "benar_salah" && question.options.length === 0
      ? [{ label: labels.trueLabel }, { label: labels.falseLabel }]
      : question.options;
  return (
    <div className="flex gap-3 pb-5 text-small text-foreground">
      <span className="w-7 shrink-0 text-right font-medium">{block.number}.</span>
      <div className="min-w-0 flex-1 space-y-2">
        <div>
          <LatexText text={question.content} />
        </div>
        {question.type === "isian" ? (
          <p className="border-b border-foreground pt-4 text-small text-muted">{labels.answerLine}</p>
        ) : (
          <ol className="space-y-1">
            {options.map((option, i) => (
              <li key={i} className="flex gap-2">
                <span className="w-5 shrink-0">{LETTERS[i] ?? i + 1}.</span>
                <span className="min-w-0 flex-1">
                  <LatexText text={option.label} />
                </span>
              </li>
            ))}
          </ol>
        )}
      </div>
    </div>
  );
}

/**
 * Off-screen printable layout of a tryout: the blocks are measured at the page's content width, flowed into A4
 * pages, and `onLaidOut` receives the page elements for capture. It only ever sees plain print data, so a question
 * renderer can change without touching the pagination.
 */
export function TryoutPrint({ title, tenantName, date, subtests, labels, onLaidOut }: TryoutPrintProps) {
  const blocks = useMemo(() => {
    const out: Block[] = [{ kind: "title", title }];
    for (const subtest of subtests) {
      out.push({ kind: "subtest", name: subtest.name, minutes: Math.round(subtest.durationSeconds / 60), count: subtest.questions.length });
      subtest.questions.forEach((question, i) => out.push({ kind: "question", number: i + 1, question }));
    }
    return out;
  }, [title, subtests]);
  const keep = useMemo(() => blocks.map((block) => block.kind !== "question"), [blocks]);

  const measureRef = useRef<HTMLDivElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const [pages, setPages] = useState<PageLayout[] | null>(null);
  const onLaidOutRef = useRef(onLaidOut);
  const imageFailedRef = useRef(labels.imageFailed);
  useLayoutEffect(() => {
    onLaidOutRef.current = onLaidOut;
    imageFailedRef.current = labels.imageFailed;
  });

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      await document.fonts.ready;
      const container = measureRef.current;
      if (!container) return;
      await inlineImages(container, imageFailedRef.current);
      if (cancelled) return;
      const heights = Array.from(container.querySelectorAll<HTMLElement>("[data-block]")).map((el) => el.getBoundingClientRect().height);
      setPages(paginate(heights, keep, BODY_H));
    })();
    return () => {
      cancelled = true;
    };
  }, [keep]);

  useEffect(() => {
    const root = rootRef.current;
    if (!pages || !root) return;
    let cancelled = false;
    void inlineImages(root, imageFailedRef.current).then(() => {
      if (!cancelled) onLaidOutRef.current(Array.from(root.querySelectorAll<HTMLElement>("[data-print-page]")));
    });
    return () => {
      cancelled = true;
    };
  }, [pages]);

  return (
    <div ref={rootRef} aria-hidden inert data-theme="light" style={{ colorScheme: "light" }} className="fixed top-0 -left-300 text-foreground">
      {pages === null ? (
        <div ref={measureRef} style={{ width: BODY_W }} className="text-foreground">
          {blocks.map((block, i) => (
            <div key={i} data-block>
              <BlockView block={block} labels={labels} />
            </div>
          ))}
        </div>
      ) : (
        pages.map((page, n) => (
          <div key={n} data-print-page className="relative overflow-hidden bg-white text-foreground" style={{ width: PAGE_W, height: PAGE_H }}>
            <div className="absolute overflow-hidden" style={{ left: PAD_X, top: PAD_TOP, width: BODY_W, height: BODY_H }}>
              {page.blocks.map(({ index, scale }) => (
                <div key={index} style={scale < 1 ? { width: BODY_W, transform: `scale(${scale})`, transformOrigin: "top center" } : undefined}>
                  <BlockView block={blocks[index]} labels={labels} />
                </div>
              ))}
            </div>
            <p className="absolute text-caption text-muted" style={{ left: PAD_X, right: PAD_X, bottom: 32 }}>
              {tenantName} · {date} · {labels.pageOf(n + 1, pages.length)}
            </p>
          </div>
        ))
      )}
    </div>
  );
}
