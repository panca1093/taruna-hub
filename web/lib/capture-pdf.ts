import { toJpeg } from "html-to-image";

const A4_WIDTH_PT = 595.28;

const bytes = (text: string) => new TextEncoder().encode(text);

/** A one-page PDF whose page is the image: A4 wide, as tall as the image needs. */
function singleImagePdf(jpeg: Uint8Array, imgW: number, imgH: number): Blob {
  const pageW = A4_WIDTH_PT;
  const pageH = Math.round(pageW * (imgH / imgW) * 100) / 100;
  const content = `q ${pageW} 0 0 ${pageH} 0 0 cm /Im0 Do Q`;
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
  object(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>");
  object(3, `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${pageW} ${pageH}] /Contents 4 0 R /Resources << /XObject << /Im0 5 0 R >> >> >>`);
  object(4, `<< /Length ${content.length} >>`, bytes(content));
  object(5, `<< /Type /XObject /Subtype /Image /Width ${imgW} /Height ${imgH} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>`, jpeg);
  const xref = size;
  push(bytes(`xref\n0 6\n0000000000 65535 f \n${offsets.slice(1).map((o) => `${String(o).padStart(10, "0")} 00000 n \n`).join("")}trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`));
  return new Blob(parts as BlobPart[], { type: "application/pdf" });
}

/** Renders the node to one PDF page exactly as it looks and saves it; `background` fills what the node leaves transparent. */
export async function savePageAsPdf(node: HTMLElement, fileName: string, background: string) {
  const pixelRatio = 2;
  const dataUrl = await toJpeg(node, { quality: 0.95, pixelRatio, backgroundColor: background, cacheBust: true });
  const jpeg = new Uint8Array(await (await fetch(dataUrl)).arrayBuffer());
  const bitmap = await createImageBitmap(new Blob([jpeg as BlobPart], { type: "image/jpeg" }));
  const { width, height } = bitmap;
  bitmap.close();
  const blob = singleImagePdf(jpeg, width, height);
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
