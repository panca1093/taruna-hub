"use client";

import type { KecermatanRuntimeElement } from "@/lib/api/client";
import { ItemElementView } from "@/components/admin/item-element";
import { cn } from "@/lib/cn";

const TEXT_KINDS = new Set(["char", "number", "symbol"]);

/** One reference item as a student sees it: text kinds read as one string, other kinds draw each element. */
export function KecermatanItem({
  elements,
  className,
  elementClassName = "size-5 text-base",
  wrap = false,
}: {
  elements: KecermatanRuntimeElement[];
  className?: string;
  elementClassName?: string;
  wrap?: boolean;
}) {
  if (elements.every((element) => TEXT_KINDS.has(element.kind))) {
    const text = elements.map((element) => element.value).join(elements[0]?.kind === "symbol" ? " " : "");
    return <span className={cn("font-mono tracking-tight text-foreground", wrap ? "break-all" : "whitespace-nowrap", className)}>{text}</span>;
  }
  return (
    <span className={cn("inline-flex items-center justify-center gap-1", wrap && "flex-wrap", className)}>
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
