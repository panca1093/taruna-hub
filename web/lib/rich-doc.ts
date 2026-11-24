import type { components } from "@/lib/api/schema";

export type RichDoc = components["schemas"]["RichDoc"];
export type RichNode = components["schemas"]["RichNode"];

export const SOLID_SHAPE_KINDS = ["kubus", "balok", "tabung", "kerucut", "bola", "limas", "prisma"] as const;
export const FLAT_SHAPE_KINDS = ["persegi", "persegi_panjang", "segitiga", "lingkaran", "trapesium", "jajar_genjang", "belah_ketupat", "layang_layang"] as const;
export const SHAPE_KINDS = [...SOLID_SHAPE_KINDS, ...FLAT_SHAPE_KINDS] as const;
export type ShapeKind = (typeof SHAPE_KINDS)[number];

/** Same bounds the API enforces on a formula; keeps a hostile LaTeX string from freezing the page. */
export const KATEX_OPTIONS = { throwOnError: false, maxSize: 10, maxExpand: 100, strict: "ignore" } as const;

export const MAX_TABLE_ROWS = 30;
export const MAX_TABLE_COLS = 12;
export const MAX_IMAGE_ALT = 200;
export const MAX_SHAPE_CAPTION = 200;
export const MAX_FORMULA_LENGTH = 2000;

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
