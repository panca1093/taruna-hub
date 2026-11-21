"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { capturePage, fontEmbedCSSOf, inlineImages, savePdf, type CapturedPage } from "@/lib/capture-pdf";

/** Off-screen light-theme box that result sheets are rendered into before they are captured as PDF pages. */
export function CaptureStage({ children, ref }: { children?: React.ReactNode; ref?: React.Ref<HTMLDivElement> }) {
  return (
    <div ref={ref} aria-hidden inert data-theme="light" style={{ colorScheme: "light" }} className="fixed top-0 -left-300 w-275">
      {children}
    </div>
  );
}

const BACKGROUND_TAB_PAINT_MS = 250;

const nextPaint = () =>
  Promise.race([
    new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))),
    new Promise<void>((resolve) => setTimeout(resolve, BACKGROUND_TAB_PAINT_MS)),
  ]);

/**
 * Builds one PDF from many result sheets, one sheet at a time: each sheet is mounted in the stage, captured as a page
 * whose height follows its content, then unmounted before the next one is loaded. `stage` must be rendered by the caller.
 */
export function useBulkSheetPdf() {
  const [sheet, setSheet] = useState<React.ReactNode>(null);
  const [progress, setProgress] = useState({ status: "idle" as "idle" | "working" | "error", page: 0, total: 0, done: 0 });
  const sheetRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => () => abortRef.current?.abort(), []);

  const stage = (
    <CaptureStage>
      {sheet && (
        <div key="sheet" ref={sheetRef}>
          {sheet}
        </div>
      )}
    </CaptureStage>
  );

  /**
   * `load` resolves the sheet of one item. Any rejection fails the whole run: no file is saved and `status` becomes
   * "error" with `done` the number of sheets captured before it failed. Resolves to whether the file was saved; an abort resolves false quietly.
   */
  const run = useCallback(
    async <T,>(items: T[], load: (item: T, signal: AbortSignal) => Promise<React.ReactNode>, fileName: string) => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      const { signal } = controller;
      const pages: CapturedPage[] = [];
      let fontEmbedCSS: string | null = null;
      try {
        for (const [i, item] of items.entries()) {
          signal.throwIfAborted();
          setProgress({ status: "working", page: i + 1, total: items.length, done: pages.length });
          const node = await load(item, signal);
          signal.throwIfAborted();
          setSheet(node);
          await nextPaint();
          const element = sheetRef.current;
          if (!element) throw new Error("sheet not mounted");
          await inlineImages(element);
          fontEmbedCSS ??= await fontEmbedCSSOf(element.parentElement ?? element);
          const background = getComputedStyle(element.firstElementChild ?? element).backgroundColor;
          pages.push(await capturePage(element, background, "content", fontEmbedCSS));
          setSheet(null);
        }
        signal.throwIfAborted();
        savePdf(pages, fileName);
        setProgress({ status: "idle", page: 0, total: 0, done: 0 });
        return true;
      } catch {
        if (!signal.aborted) setProgress((p) => ({ ...p, status: "error", done: pages.length }));
        return false;
      } finally {
        setSheet(null);
      }
    },
    [],
  );

  return { stage, run, ...progress };
}
