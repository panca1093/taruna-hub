"use client";

import type { KecermatanRuntimeElement } from "@/lib/api/client";
import { ItemElementView } from "@/components/admin/item-element";
import { cn } from "@/lib/cn";

const TEXT_KINDS = new Set(["char", "number", "symbol"]);

/** One reference item as a student sees it: text kinds read as one string, other kinds draw each element. `fit` lets non-text items wrap between elements and drops the spaces between symbols when the card is narrow; a text run never splits, so callers shrink it. */
export function KecermatanItem({
  elements,
  className,
  elementClassName = "size-5 text-base",
  fit = false,
}: {
  elements: KecermatanRuntimeElement[];
  className?: string;
  elementClassName?: string;
  fit?: boolean;
}) {
  if (elements.every((element) => TEXT_KINDS.has(element.kind))) {
    const isSymbol = elements[0]?.kind === "symbol";
    const compact = elements.map((element) => element.value).join("");
    if (fit && isSymbol) {
      const spaced = elements.map((element) => element.value).join(" ");
      return (
        <span className={cn("font-mono tracking-tight whitespace-nowrap text-foreground", className)}>
          <span className="hidden @item-md:inline">{spaced}</span>
          <span className="item-symbol-tight @item-md:hidden">{compact}</span>
        </span>
      );
    }
    const text = isSymbol ? elements.map((element) => element.value).join(" ") : compact;
    return <span className={cn("font-mono tracking-tight whitespace-nowrap text-foreground", className)}>{text}</span>;
  }
  return (
    <span className={cn("inline-flex items-center justify-center gap-1", fit && "flex-wrap", className)}>
      {elements.map((element, index) =>
        TEXT_KINDS.has(element.kind) ? (
          <span key={`${element.kind}:${element.value}:${index}`} className="font-mono leading-none text-foreground">
            {element.value}
          </span>
        ) : (
          <ItemElementView key={`${element.kind}:${element.value}:${index}`} element={{ ...element, asset_id: element.asset_id ?? undefined }} className={elementClassName} />
        ),
      )}
    </span>
  );
}
