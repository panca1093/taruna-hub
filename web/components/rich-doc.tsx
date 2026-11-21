"use client";

import { Fragment, useEffect, useState, type ReactNode } from "react";
import katex from "katex";
import "katex/dist/katex.min.css";
import { api } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useRichDocT } from "@/lib/i18n/dict/rich-doc";
import { KATEX_OPTIONS, SHAPE_KINDS, type RichDoc as RichDocValue, type RichNode, type ShapeKind } from "@/lib/rich-doc";
import { SolidShape } from "@/components/solid-shape";

const isShapeKind = (kind: unknown): kind is ShapeKind => (SHAPE_KINDS as readonly unknown[]).includes(kind);
const asSpan = (value: unknown) => (typeof value === "number" && Number.isInteger(value) && value > 0 ? value : undefined);

function RichImage({ attrs }: { attrs: Record<string, unknown> }) {
  const t = useRichDocT();
  const assetId = typeof attrs.asset_id === "string" ? attrs.asset_id : "";
  const legacySrc = !assetId && typeof attrs.src === "string" && /^https?:\/\//.test(attrs.src) ? attrs.src : "";
  const alt = typeof attrs.alt === "string" ? attrs.alt : "";
  const [link, setLink] = useState<{ id: string; url: string | null } | null>(null);
  const [loaded, setLoaded] = useState<{ url: string; ok: boolean } | null>(null);

  useEffect(() => {
    if (!assetId) return;
    let cancelled = false;
    api.assets
      .url(assetId)
      .then((l) => !cancelled && setLink({ id: assetId, url: l.url }))
      .catch(() => !cancelled && setLink({ id: assetId, url: null }));
    return () => {
      cancelled = true;
    };
  }, [assetId]);

  // undefined while the link is pending, null when it is unusable
  const url = assetId ? (link?.id === assetId ? link.url : undefined) : legacySrc || null;
  const result = url && loaded?.url === url ? loaded.ok : undefined;
  const state = url === null || result === false ? "error" : result ? "true" : undefined;

  return (
    <figure className="rich-image" data-rich-image data-loaded={state}>
      {url && (
        // eslint-disable-next-line @next/next/no-img-element -- a short-lived presigned URL cannot go through the optimizer
        <img src={url} alt={alt} data-asset-id={assetId || undefined} onLoad={() => setLoaded({ url, ok: true })} onError={() => setLoaded({ url, ok: false })} />
      )}
      {state === undefined && <span role="status" aria-label={t.imageLoading} className="block h-24 w-full animate-pulse rounded-chip bg-surface-secondary" />}
      {state === "error" && <span role="status">{t.imageFailed}</span>}
    </figure>
  );
}

function RichShape({ kind, caption }: { kind: ShapeKind; caption: string }) {
  const t = useRichDocT();
  return (
    <figure className="rich-shape">
      <SolidShape kind={kind} label={caption || t.shapes[kind]} />
      {caption && <figcaption>{caption}</figcaption>}
    </figure>
  );
}

function renderText(node: RichNode, key: number): ReactNode {
  let out: ReactNode = node.text ?? "";
  const marks = new Set((node.marks ?? []).map((m) => m.type));
  if (marks.has("underline")) out = <u>{out}</u>;
  if (marks.has("italic")) out = <em>{out}</em>;
  if (marks.has("bold")) out = <strong>{out}</strong>;
  return <Fragment key={key}>{out}</Fragment>;
}

const renderChildren = (node: RichNode): ReactNode[] => (node.content ?? []).map((child, i) => renderNode(child, i));

function renderNode(node: RichNode, key: number): ReactNode {
  const attrs = node.attrs ?? {};
  switch (node.type) {
    case "text":
      return renderText(node, key);
    case "hardBreak":
      return <br key={key} />;
    case "paragraph": {
      const children = renderChildren(node);
      return <p key={key}>{children.length > 0 ? children : <br />}</p>;
    }
    case "bulletList":
      return <ul key={key}>{renderChildren(node)}</ul>;
    case "orderedList":
      return (
        <ol key={key} start={asSpan(attrs.start)}>
          {renderChildren(node)}
        </ol>
      );
    case "listItem":
      return <li key={key}>{renderChildren(node)}</li>;
    case "table":
      return (
        <div key={key} className="rich-table">
          <table>
            <tbody>{renderChildren(node)}</tbody>
          </table>
        </div>
      );
    case "tableRow":
      return <tr key={key}>{renderChildren(node)}</tr>;
    case "tableHeader":
      return (
        <th key={key} colSpan={asSpan(attrs.colspan)} rowSpan={asSpan(attrs.rowspan)}>
          {renderChildren(node)}
        </th>
      );
    case "tableCell":
      return (
        <td key={key} colSpan={asSpan(attrs.colspan)} rowSpan={asSpan(attrs.rowspan)}>
          {renderChildren(node)}
        </td>
      );
    case "math": {
      const latex = typeof attrs.latex === "string" ? attrs.latex : "";
      return <span key={key} className="rich-math" dangerouslySetInnerHTML={{ __html: katex.renderToString(latex, KATEX_OPTIONS) }} />;
    }
    case "shape": {
      if (!isShapeKind(attrs.kind)) return null;
      return <RichShape key={key} kind={attrs.kind} caption={typeof attrs.caption === "string" ? attrs.caption : ""} />;
    }
    case "image":
      return <RichImage key={key} attrs={attrs} />;
    default:
      return null;
  }
}

export function RichDoc({ doc, className }: { doc: RichDocValue; className?: string }) {
  return <div className={cn("rich-text", className)}>{(doc?.content ?? []).map((node, i) => renderNode(node, i))}</div>;
}
