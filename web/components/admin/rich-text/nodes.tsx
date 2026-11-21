"use client";

import { useEffect, useState } from "react";
import { Node, NodeViewWrapper, ReactNodeViewRenderer, type ReactNodeViewProps } from "@tiptap/react";
import katex from "katex";
import "katex/dist/katex.min.css";
import { SolidShape } from "@/components/solid-shape";
import { api } from "@/lib/api/client";
import { SHAPE_KINDS, type ShapeKind } from "@/lib/rich-doc";
import { useAdminRichTextEditorT } from "@/lib/i18n/dict/admin-rich-text-editor";
import { useRichTextGroup } from "./group";

const MathView = ({ node, getPos }: ReactNodeViewProps) => {
  const t = useAdminRichTextEditorT();
  const group = useRichTextGroup();
  const latex = String(node.attrs.latex ?? "");
  return (
    <NodeViewWrapper as="span" className="rich-math cursor-pointer" title={t.mathEditHint}>
      <span
        onClick={() => {
          const pos = getPos();
          if (pos !== undefined) group.openPopover("math", { pos, latex });
        }}
        dangerouslySetInnerHTML={{ __html: katex.renderToString(latex, { throwOnError: false }) }}
      />
    </NodeViewWrapper>
  );
};

const ShapeView = ({ node }: ReactNodeViewProps) => {
  const kind = SHAPE_KINDS.includes(node.attrs.kind) ? (node.attrs.kind as ShapeKind) : SHAPE_KINDS[0];
  const caption = String(node.attrs.caption ?? "");
  return (
    <NodeViewWrapper as="figure" className="rich-shape" data-drag-handle="">
      <SolidShape kind={kind} />
      {caption && <figcaption>{caption}</figcaption>}
    </NodeViewWrapper>
  );
};

const ImageView = ({ node }: ReactNodeViewProps) => {
  const t = useAdminRichTextEditorT();
  const assetId = (node.attrs.asset_id as string | null) ?? null;
  const [resolved, setResolved] = useState<{ id: string; url: string | null } | null>(null);

  useEffect(() => {
    if (!assetId) return;
    let cancelled = false;
    api.assets
      .url(assetId)
      .then(({ url }) => !cancelled && setResolved({ id: assetId, url }))
      .catch(() => !cancelled && setResolved({ id: assetId, url: null }));
    return () => {
      cancelled = true;
    };
  }, [assetId]);

  const settled = assetId !== null && resolved?.id === assetId ? resolved : null;
  const src = assetId ? (settled?.url ?? null) : ((node.attrs.src as string | null) ?? null);
  const failed = settled !== null && settled.url === null;
  return (
    <NodeViewWrapper as="figure" className="rich-image" data-drag-handle="">
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element -- presigned asset URL, not optimizable
        <img src={src} alt={String(node.attrs.alt ?? "")} />
      ) : (
        failed && <span className="text-caption text-muted">{t.imageLoadFailed}</span>
      )}
    </NodeViewWrapper>
  );
};

export const MathNode = Node.create({
  name: "math",
  group: "inline",
  inline: true,
  atom: true,
  addAttributes() {
    return { latex: { default: "" } };
  },
  parseHTML() {
    return [{ tag: "span[data-math]", getAttrs: (el) => ({ latex: (el as HTMLElement).getAttribute("data-math") ?? "" }) }];
  },
  renderHTML({ node }) {
    return ["span", { "data-math": node.attrs.latex }];
  },
  addNodeView() {
    return ReactNodeViewRenderer(MathView, { as: "span" });
  },
});

export const ShapeNode = Node.create({
  name: "shape",
  group: "block",
  atom: true,
  draggable: true,
  addAttributes() {
    return { kind: { default: "kubus" }, caption: { default: "" } };
  },
  parseHTML() {
    return [
      {
        tag: "figure[data-shape]",
        getAttrs: (el) => ({ kind: (el as HTMLElement).dataset.shape, caption: (el as HTMLElement).dataset.caption ?? "" }),
      },
    ];
  },
  renderHTML({ node }) {
    return ["figure", { "data-shape": node.attrs.kind, "data-caption": node.attrs.caption }];
  },
  addNodeView() {
    return ReactNodeViewRenderer(ShapeView, { as: "figure" });
  },
});

export const ImageNode = Node.create({
  name: "image",
  group: "block",
  atom: true,
  draggable: true,
  addAttributes() {
    return { asset_id: { default: null }, alt: { default: "" }, src: { default: null } };
  },
  parseHTML() {
    return [
      {
        tag: "figure[data-image]",
        getAttrs: (el) => {
          const d = (el as HTMLElement).dataset;
          return { asset_id: d.assetId || null, alt: d.alt ?? "", src: d.src || null };
        },
      },
    ];
  },
  renderHTML({ node }) {
    const { asset_id, alt, src } = node.attrs;
    return ["figure", { "data-image": "", "data-asset-id": asset_id ?? "", "data-alt": alt ?? "", "data-src": src ?? "" }];
  },
  addNodeView() {
    return ReactNodeViewRenderer(ImageView, { as: "figure" });
  },
});
