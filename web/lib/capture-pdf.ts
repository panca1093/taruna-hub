import { getFontEmbedCSS, toJpeg } from "html-to-image";

const A4_WIDTH_PT = 595.28;
const A4_HEIGHT_PT = 841.89;

const bytes = (text: string) => new TextEncoder().encode(text);

interface PdfPage {
  jpeg: Uint8Array;
  imgW: number;
  imgH: number;
  pageW: number;
  pageH: number;
}

/** A PDF with one page per image; each page is exactly the size it is given and the image fills it. */
function imagePdf(pages: PdfPage[]): Blob {
  const parts: Uint8Array[] = [];
  const offsets: number[] = [];
  let size = 0;
  const push = (chunk: Uint8Array) => {
    parts.push(chunk);
    size += chunk.length;
  };
  const object = (n: number, head: string, stream?: Uint8Array) => {
    offsets[n] = size;
    push(bytes(`${n} 0 obj\n${head}\n`));
    if (stream) {
      push(bytes("stream\n"));
      push(stream);
      push(bytes("\nendstream\n"));
    }
    push(bytes("endobj\n"));
  };
  push(bytes("%PDF-1.4\n"));
  object(1, "<< /Type /Catalog /Pages 2 0 R >>");
  object(2, `<< /Type /Pages /Kids [${pages.map((_, i) => `${3 + i * 3} 0 R`).join(" ")}] /Count ${pages.length} >>`);
  pages.forEach(({ jpeg, imgW, imgH, pageW, pageH }, i) => {
    const page = 3 + i * 3;
    const content = `q ${pageW} 0 0 ${pageH} 0 0 cm /Im0 Do Q`;
    object(page, `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${pageW} ${pageH}] /Contents ${page + 1} 0 R /Resources << /XObject << /Im0 ${page + 2} 0 R >> >> >>`);
    object(page + 1, `<< /Length ${content.length} >>`, bytes(content));
    object(page + 2, `<< /Type /XObject /Subtype /Image /Width ${imgW} /Height ${imgH} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>`, jpeg);
  });
  const count = 3 + pages.length * 3;
  const xref = size;
  push(bytes(`xref\n0 ${count}\n0000000000 65535 f \n${offsets.slice(1).map((o) => `${String(o).padStart(10, "0")} 00000 n \n`).join("")}trailer\n<< /Size ${count} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`));
  return new Blob(parts as BlobPart[], { type: "application/pdf" });
}

async function captureJpeg(node: HTMLElement, background: string, fontEmbedCSS: string) {
  const dataUrl = await toJpeg(node, { quality: 0.95, pixelRatio: 2, backgroundColor: background, cacheBust: false, fontEmbedCSS });
  const jpeg = new Uint8Array(await (await fetch(dataUrl)).arrayBuffer());
  const bitmap = await createImageBitmap(new Blob([jpeg as BlobPart], { type: "image/jpeg" }));
  const { width, height } = bitmap;
  bitmap.close();
  return { jpeg, imgW: width, imgH: height };
}

function download(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** "a4" gives every page the fixed A4 portrait size; "content" keeps the A4 width and lets each page's height follow its element. */
export type PdfPageSize = "a4" | "content";

const pageSizeOf = (shot: { imgW: number; imgH: number }, size: PdfPageSize) =>
  size === "a4"
    ? { pageW: A4_WIDTH_PT, pageH: A4_HEIGHT_PT }
    : { pageW: A4_WIDTH_PT, pageH: Math.round(A4_WIDTH_PT * (shot.imgH / shot.imgW) * 100) / 100 };

export type CapturedPage = PdfPage;

/** CSS of the web fonts used under `node`, computed once and passed to every `capturePage` of a run. */
export async function fontEmbedCSSOf(node: HTMLElement) {
  await document.fonts.ready;
  return getFontEmbedCSS(node);
}

/** Captures one element as a page that stays in memory until `savePdf` writes the file. */
export async function capturePage(node: HTMLElement, background: string, size: PdfPageSize, fontEmbedCSS: string): Promise<CapturedPage> {
  const shot = await captureJpeg(node, background, fontEmbedCSS);
  return { ...shot, ...pageSizeOf(shot, size) };
}

export function savePdf(pages: CapturedPage[], fileName: string) {
  download(imagePdf(pages), fileName);
}

/** Renders the node to one PDF page exactly as it looks and saves it; `background` fills what the node leaves transparent. */
export async function savePageAsPdf(node: HTMLElement, fileName: string, background: string) {
  await inlineImages(node);
  savePdf([await capturePage(node, background, "content", await fontEmbedCSSOf(node))], fileName);
}

/**
 * Captures each element as one page of a single PDF, one after the other so only one bitmap is decoded at
 * a time (the JPEG bytes of every page are kept until the file is written). Pages are A4 portrait.
 * `onProgress` gets the 1-based page about to be captured and the total; an aborted `signal` stops before the next
 * page and before the download.
 */
export async function savePagesAsPdf(
  pages: HTMLElement[],
  fileName: string,
  background: string,
  onProgress?: (page: number, total: number) => void,
  signal?: AbortSignal,
) {
  // Fonts are collected from the pages' shared parent: page 1 alone may not use the math faces later pages need.
  const fontEmbedCSS = pages.length > 0 ? await fontEmbedCSSOf(pages[0].parentElement ?? pages[0]) : "";
  const shots: CapturedPage[] = [];
  for (const [i, node] of pages.entries()) {
    signal?.throwIfAborted();
    onProgress?.(i + 1, pages.length);
    shots.push(await capturePage(node, background, "a4", fontEmbedCSS));
  }
  signal?.throwIfAborted();
  savePdf(shots, fileName);
}

function readAsDataUrl(blob: Blob) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(blob);
  });
}

const IMAGE_TIMEOUT_MS = 10_000;

async function loadedOk(img: HTMLImageElement) {
  if (!img.complete) {
    await new Promise<void>((resolve) => {
      img.addEventListener("load", () => resolve(), { once: true });
      img.addEventListener("error", () => resolve(), { once: true });
      setTimeout(resolve, IMAGE_TIMEOUT_MS);
    });
  }
  return img.complete && img.naturalWidth > 0;
}

// html-to-image fetches every <img> src while cloning and rejects the whole capture when one fails, so a replaced image gets a src that never needs fetching.
const BLANK_PIXEL = "data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==";

const withTimeout = <T>(promise: Promise<T>) =>
  Promise.race([promise, new Promise<never>((_, reject) => setTimeout(() => reject(new Error("image timed out")), IMAGE_TIMEOUT_MS))]);

/**
 * Makes every image under `root` self-contained before a capture: images become `data:` URLs, and one that fails
 * (to load, to fetch, or in `resolve`, which may supply the bytes itself) is hidden behind a bordered box with its
 * alt text (or `fallback`), so it never prints as a gap. It never rejects.
 */
export async function inlineImages(root: HTMLElement, fallback = "", resolve?: (img: HTMLImageElement) => Promise<Blob> | null) {
  await Promise.all(
    Array.from(root.querySelectorAll("img")).map(async (img) => {
      try {
        const supplied = resolve?.(img) ?? null;
        let blob: Blob;
        if (supplied) {
          blob = await withTimeout(supplied);
        } else {
          if (!(await loadedOk(img))) throw new Error("image failed to load");
          const src = img.currentSrc || img.src;
          if (src.startsWith("data:")) return;
          const response = await fetch(src, { signal: AbortSignal.timeout(IMAGE_TIMEOUT_MS) });
          if (!response.ok) throw new Error(`image fetch ${response.status}`);
          blob = await response.blob();
        }
        const dataUrl = await readAsDataUrl(blob);
        img.dataset.inlined = "1";
        img.removeAttribute("srcset");
        img.src = dataUrl;
        await img.decode();
      } catch {
        const box = document.createElement("span");
        box.className = "inline-block rounded-lg border border-border px-2 py-1 text-caption text-muted";
        box.textContent = img.alt || fallback;
        // The image stays in the tree, hidden, because React still owns that node.
        img.hidden = true;
        img.dataset.inlined = "1";
        img.removeAttribute("srcset");
        img.src = BLANK_PIXEL;
        img.after(box);
      }
    }),
  );
}
