import type { RichDoc as RichDocValue } from "@/lib/rich-doc";
import { docText } from "@/lib/rich-doc";

// TODO: uncertain — placeholder body so dependants compile; the real node renderer replaces it before this ships.
export function RichDoc({ doc, className }: { doc: RichDocValue; className?: string }) {
  return <div className={className}>{docText(doc)}</div>;
}
