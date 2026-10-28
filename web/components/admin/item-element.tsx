"use client";

import { useEffect, useState } from "react";
import { api, type ItemElement } from "@/lib/api/client";
import { cn } from "@/lib/cn";

const shapePaths: Record<string, React.ReactNode> = {
  kotak: <rect x="4" y="4" width="16" height="16" />,
  segitiga: <polygon points="12,3 21,20 3,20" />,
  persegi_panjang: <rect x="2" y="6" width="20" height="12" />,
  lingkaran: <circle cx="12" cy="12" r="9" />,
  jajar_genjang: <polygon points="7,5 22,5 17,19 2,19" />,
  trapesium: <polygon points="7,5 17,5 22,19 2,19" />,
  belah_ketupat: <polygon points="12,2 22,12 12,22 2,12" />,
};

export function ShapeIcon({ shape, className }: { shape: string; className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden className={cn("size-6", className)}>
      {shapePaths[shape]}
    </svg>
  );
}

export function EmojiImage({ id, label, className }: { id: string; label: string; className?: string }) {
  // Bundled Twemoji asset, so the picture is the same on every device.
  // eslint-disable-next-line @next/next/no-img-element -- a static SVG gains nothing from the optimizer
  return <img src={`/emoji/${id}.svg`} alt={label} className={cn("size-6", className)} />;
}

// Presigned links last about a minute; sharing one promise for 45 seconds spares a request per grid cell on re-render.
const linkCache = new Map<string, { link: Promise<string>; at: number }>();

function assetLink(assetId: string): Promise<string> {
  const cached = linkCache.get(assetId);
  if (cached && Date.now() - cached.at < 45_000) return cached.link;
  const link = api.assets.url(assetId).then((l) => l.url);
  link.catch(() => linkCache.delete(assetId));
  linkCache.set(assetId, { link, at: Date.now() });
  return link;
}

function useAssetUrl(assetId: string | undefined) {
  const [state, setState] = useState<{ id: string; url: string | null; failed: boolean } | null>(null);
  useEffect(() => {
    if (!assetId) return;
    let cancelled = false;
    assetLink(assetId)
      .then((url) => {
        if (!cancelled) setState({ id: assetId, url, failed: false });
      })
      .catch(() => {
        if (!cancelled) setState({ id: assetId, url: null, failed: true });
      });
    return () => {
      cancelled = true;
    };
  }, [assetId]);
  return state && state.id === assetId ? state : null;
}

/** What a view needs; package elements and generated kecermatan elements both fit. */
export type ViewElement = Pick<ItemElement, "value" | "asset_id"> & { kind: string; label?: string };

function ImageElement({ element, className }: { element: ViewElement; className?: string }) {
  const link = useAssetUrl(element.asset_id);
  if (!link?.url) {
    return <span className={cn("block size-10 rounded-md bg-surface-secondary", !link?.failed && "animate-pulse", className)} />;
  }
  // eslint-disable-next-line @next/next/no-img-element -- a short-lived presigned URL cannot go through the optimizer
  return <img src={link.url} alt={element.label ?? ""} className={cn("size-10 rounded-md object-contain", className)} />;
}

/** One element of a package revision, drawn the way a student will see it. */
export function ItemElementView({ element, className }: { element: ViewElement; className?: string }) {
  switch (element.kind) {
    case "char":
    case "number":
    case "symbol":
      return <span className={cn("text-2xl leading-none text-foreground", className)}>{element.value}</span>;
    case "emoji":
      return <EmojiImage id={element.value} label={element.label ?? ""} className={className} />;
    case "shape":
      return <ShapeIcon shape={element.value} className={cn("text-foreground", className)} />;
    default:
      return <ImageElement element={element} className={className} />;
  }
}
