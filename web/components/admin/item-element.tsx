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
  segi_lima: <polygon points="12.0,1.5 22.0,8.8 18.2,20.5 5.8,20.5 2.0,8.8" />,
  segi_enam: <polygon points="12.0,2.0 20.7,7.0 20.7,17.0 12.0,22.0 3.3,17.0 3.3,7.0" />,
  segi_delapan: <polygon points="15.8,2.8 21.2,8.2 21.2,15.8 15.8,21.2 8.2,21.2 2.8,15.8 2.8,8.2 8.2,2.8" />,
  bintang: <polygon points="12.0,2.1 14.6,9.0 22.0,9.4 16.3,14.0 18.2,21.1 12.0,17.1 5.8,21.1 7.7,14.0 2.0,9.4 9.4,9.0" />,
  oval: <ellipse cx="12" cy="12" rx="10" ry="6.5" />,
  setengah_lingkaran: <path d="M2 17 A10 10 0 0 1 22 17 Z" />,
  layang_layang: <polygon points="12,2 19,9 12,22 5,9" />,
  tanda_tambah: <polygon points="9,3 15,3 15,9 21,9 21,15 15,15 15,21 9,21 9,15 3,15 3,9 9,9" />,
  hati: <path d="M12 21 C4 15 2 11 3.5 7.5 C5 4 9.5 4 12 8 C14.5 4 19 4 20.5 7.5 C22 11 20 15 12 21 Z" />,
  panah: <polygon points="2,9 13,9 13,4 22,12 13,20 13,15 2,15" />,
  kubus: (
    <>
      <rect x="3" y="9" width="12" height="12" />
      <polygon points="3,9 9,3 21,3 15,9" fillOpacity={0.55} />
      <polygon points="15,9 21,3 21,15 15,21" fillOpacity={0.3} />
    </>
  ),
  balok: (
    <>
      <rect x="2" y="11" width="14" height="10" />
      <polygon points="2,11 8,5 22,5 16,11" fillOpacity={0.55} />
      <polygon points="16,11 22,5 22,15 16,21" fillOpacity={0.3} />
    </>
  ),
  tabung: (
    <>
      <path d="M5 5 V19 A7 2.5 0 0 0 19 19 V5 Z" />
      <ellipse cx="12" cy="5" rx="7" ry="2.5" fillOpacity={0.55} />
    </>
  ),
  kerucut: (
    <>
      <path d="M12 2 L20 19 A8 2.5 0 0 1 4 19 Z" />
      <ellipse cx="12" cy="19" rx="8" ry="2.5" fillOpacity={0.55} />
    </>
  ),
  bola: (
    <>
      <circle cx="12" cy="12" r="10" fillOpacity={0.75} />
      <path d="M2 12 A10 3.5 0 0 0 22 12 A10 10 0 0 1 2 12 Z" />
    </>
  ),
  limas: (
    <>
      <polygon points="12,2 2,18 12,22" />
      <polygon points="12,2 12,22 22,18" fillOpacity={0.55} />
    </>
  ),
  prisma: (
    <>
      <polygon points="2,21 8,9 14,21" />
      <polygon points="8,9 17,3 22,15 14,21" fillOpacity={0.45} />
    </>
  ),
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
    return <span className={cn("block size-10 rounded-chip bg-surface-secondary", !link?.failed && "animate-pulse", className)} />;
  }
  // eslint-disable-next-line @next/next/no-img-element -- a short-lived presigned URL cannot go through the optimizer
  return <img src={link.url} alt={element.label ?? ""} className={cn("size-10 rounded-chip object-contain", className)} />;
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
