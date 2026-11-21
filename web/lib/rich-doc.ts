import type { components } from "@/lib/api/schema";

export type RichDoc = components["schemas"]["RichDoc"];
export type RichNode = components["schemas"]["RichNode"];

export const SHAPE_KINDS = ["kubus", "balok", "tabung", "kerucut", "bola", "limas", "prisma"] as const;
export type ShapeKind = (typeof SHAPE_KINDS)[number];

export const emptyDoc = (): RichDoc => ({ type: "doc", content: [] });

export function docFromText(text: string): RichDoc {
  const lines = text.split("\n");
  return {
    type: "doc",
    content: text === "" ? [] : lines.map((line) => ({ type: "paragraph" as const, content: line === "" ? [] : [{ type: "text" as const, text: line }] })),
  };
}

const ATOMS = new Set<RichNode["type"]>(["math", "shape", "image"]);

function hasContent(node: RichNode): boolean {
  if (node.type === "text") return (node.text ?? "").trim() !== "";
  if (ATOMS.has(node.type)) return true;
  return (node.content ?? []).some(hasContent);
}

/** True when the document shows nothing: no text, formula, shape or image. */
export const isDocEmpty = (doc: RichDoc | null | undefined) => !doc || !doc.content.some(hasContent);

function nodeText(node: RichNode): string {
  if (node.type === "text") return node.text ?? "";
  if (node.type === "math") return String(node.attrs?.latex ?? "");
  if (node.type === "hardBreak") return " ";
  return (node.content ?? []).map(nodeText).join(node.type === "paragraph" ? "" : " ");
}

/** Plain text of a document, for titles, aria labels and sorting; formulas appear as their LaTeX. */
export const docText = (doc: RichDoc | null | undefined) =>
  (doc?.content ?? []).map(nodeText).join(" ").replace(/\s+/g, " ").trim();
