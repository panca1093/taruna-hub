/**
 * Tryout Center — PRD & RAB PDF Export
 *
 * Renders ../PRD.md and ../RAB.md into styled, print-ready PDFs.
 *
 * Setup (one-time):
 *   cd export-mockup
 *   npm install
 *   npm run setup
 *
 * Run:
 *   npm run export:docs
 *
 * Output:
 *   ./product-requirements.pdf
 *   ./rab.pdf
 */

import { chromium } from "playwright";
import { marked } from "marked";
import { readFileSync, writeFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");
const PRD_MD = join(ROOT, "PRD.md");
const OUT_PRD = join(__dirname, "product-requirements.pdf");
const RAB_MD = join(ROOT, "RAB.md");
const OUT_RAB = join(__dirname, "rab.pdf");

// ─── Styling — matches the app's indigo → violet design system ─────────────

const PRD_CSS = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

  @page { size: A4; margin: 18mm 20mm 20mm; }
  * { box-sizing: border-box; }

  body {
    font-family: 'Inter', 'Helvetica Neue', Arial, sans-serif;
    font-size: 9pt;
    color: #1a1a1a;
    line-height: 1.35;
  }

  h1 {
    font-size: 15pt;
    color: #1e1b3a;
    margin: 0 0 4px;
    padding-bottom: 8px;
    border-bottom: 3px solid #4f46e5;
    page-break-after: avoid;
  }
  h1 + h2 {
    font-size: 11pt;
    font-weight: 500;
    color: #6b7280;
    background: none;
    padding: 0;
    margin: 0 0 20px;
    border-radius: 0;
  }
  h2 {
    font-size: 10.5pt;
    color: #ffffff;
    background: linear-gradient(135deg, #4f46e5, #8b5cf6);
    margin: 22px 0 0;
    padding: 6px 11px;
    border-radius: 4px 4px 0 0;
  }
  h3 {
    font-size: 9.5pt;
    color: #1e1b3a;
    border-left: 4px solid #4f46e5;
    padding-left: 9px;
    margin: 18px 0 5px;
  }
  h4 {
    font-size: 9pt;
    color: #444;
    margin: 10px 0 3px;
  }

  p  { margin: 6px 0; }
  ul, ol { padding-left: 18px; margin: 4px 0; }
  li { margin: 3px 0; }
  /* Keep a nested sub-list (e.g. "Individual" / "Bundle" under one point)
     glued to its parent bullet — these are always short in this doc. */
  li:has(> ul), li:has(> ol) { page-break-inside: avoid; break-inside: avoid; }
  strong { color: #1e1b3a; }
  a { color: #4f46e5; text-decoration: none; }

  code {
    background: #f0f0fb;
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 7.5pt;
    font-family: 'Courier New', monospace;
    color: #3730a3;
  }
  pre {
    background: #f3f4f8;
    padding: 8px 11px;
    border-radius: 5px;
    margin: 8px 0;
    overflow-x: auto;
  }
  pre code { background: none; padding: 0; font-size: 7pt; color: #333; white-space: pre; }

  table {
    width: 100%;
    border-collapse: collapse;
    margin: 0 0 14px;
    font-size: 8.5pt;
  }
  /* Tables are allowed to break across pages — the header repeats via
     table-header-group below — but never mid-row. */
  thead { display: table-header-group; }
  th {
    background: #4f46e5;
    color: #ffffff;
    padding: 5px 8px;
    text-align: left;
    font-weight: 600;
    font-size: 8pt;
  }
  td {
    padding: 4px 8px;
    border-bottom: 1px solid #e5e7eb;
    vertical-align: top;
  }
  tr { page-break-inside: avoid; break-inside: avoid; }
  tr:nth-child(even) td { background: #f8f8fd; }

  blockquote {
    border-left: 4px solid #4f46e5;
    margin: 12px 0;
    padding: 8px 14px;
    color: #444;
    background: #f2f1fc;
    border-radius: 0 6px 6px 0;
    page-break-inside: avoid;
    break-inside: avoid;
  }

  hr { border: none; border-top: 2px solid #e5e4f5; margin: 22px 0; }

  .cover-meta {
    margin: 14px 0 22px;
    padding: 12px 14px;
    background: #f2f1fc;
    border-radius: 8px;
    font-size: 8.5pt;
    color: #444;
  }
  .cover-meta p { margin: 2px 0; }
`;

/** Pulls the leading run of "**Label:** value" lines out into a meta card. */
function extractCoverMeta(md) {
  const match = md.match(/^\*\*[^*\n]+:\*\*.*(?:\n\*\*[^*\n]+:\*\*.*)*/m);
  if (!match) return { metaLines: [], rest: md };
  const metaLines = match[0].split("\n").filter(Boolean);
  const rest = md.slice(0, match.index) + md.slice(match.index + match[0].length);
  return { metaLines, rest };
}

// Explicit page-break points, keyed by exact heading text as it appears in
// PRD.md (marked emits plain "<h2>text</h2>" — no ids/attributes). Sections
// not listed here just flow onto whatever page has room, so pairs like
// "1 & 2" or "6 & 7" share a page naturally when they fit.
const PAGE_BREAK_BEFORE = [
  { tag: "h2", text: "3. Target Users &amp; Personas" },
  { tag: "h2", text: "5. Functional Requirements" },
  { tag: "h3", text: "5.4 Pengerjaan Tryout" },
  { tag: "h3", text: "5.6 Sistem Penilaian" },
  { tag: "h3", text: "5.10 Admin &amp; Operator — Bank Soal" },
  { tag: "h3", text: "5.19 Landing Page (Marketing)" },
  { tag: "h2", text: "6. Non-Functional Requirements" },
  { tag: "h3", text: "7.1 Async &amp; Event Architecture" },
  { tag: "h2", text: "8. Data Model" },
  { tag: "h2", text: "9. Roadmap" },
];

function processPrdHtml(html) {
  const pb = '<div style="break-before: page; page-break-before: always;"></div>';
  let result = html;
  for (const { tag, text } of PAGE_BREAK_BEFORE) {
    const marker = `<${tag}>${text}</${tag}>`;
    if (!result.includes(marker)) {
      console.warn(`  ⚠ page-break marker not found for: ${text}`);
      continue;
    }
    result = result.replace(marker, pb + marker);
  }
  return result;
}

async function buildPrdPdf(page) {
  console.log("\n  [PRD] Rendering PRD.md…");
  const rawMd = readFileSync(PRD_MD, "utf8");
  const { metaLines, rest } = extractCoverMeta(rawMd);

  const bodyHtml = processPrdHtml(marked.parse(rest));
  const metaHtml =
    metaLines.length > 0
      ? `<div class="cover-meta">${metaLines.map((l) => `<p>${marked.parseInline(l)}</p>`).join("")}</div>`
      : "";

  const fullHtml = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8" />
<link rel="preconnect" href="https://fonts.googleapis.com" />
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
<style>${PRD_CSS}</style>
</head>
<body>${metaHtml}${bodyHtml}</body>
</html>`;

  await page.setContent(fullHtml, { waitUntil: "networkidle" });

  const buf = await page.pdf({
    format: "A4",
    printBackground: true,
    margin: { top: "18mm", right: "20mm", bottom: "20mm", left: "20mm" },
  });
  writeFileSync(OUT_PRD, buf);
  console.log(`  ✅  PRD PDF → ${OUT_PRD}`);
}

async function buildRabPdf(page) {
  console.log("\n  [RAB] Rendering RAB.md…");
  const rawMd = readFileSync(RAB_MD, "utf8");
  const { metaLines, rest } = extractCoverMeta(rawMd);

  const bodyHtml = marked.parse(rest);
  const metaHtml =
    metaLines.length > 0
      ? `<div class="cover-meta">${metaLines.map((l) => `<p>${marked.parseInline(l)}</p>`).join("")}</div>`
      : "";

  const fullHtml = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8" />
<link rel="preconnect" href="https://fonts.googleapis.com" />
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
<style>${PRD_CSS}</style>
</head>
<body>${metaHtml}${bodyHtml}</body>
</html>`;

  await page.setContent(fullHtml, { waitUntil: "networkidle" });

  const buf = await page.pdf({
    format: "A4",
    printBackground: true,
    margin: { top: "18mm", right: "20mm", bottom: "20mm", left: "20mm" },
  });
  writeFileSync(OUT_RAB, buf);
  console.log(`  ✅  RAB PDF → ${OUT_RAB}`);
}

async function main() {
  console.log("\n▶  Tryout Center — Docs Export");
  const browser = await chromium.launch();
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on("console", () => {});

  try {
    await buildPrdPdf(page);
    await buildRabPdf(page);
    console.log("\n  Done.\n");
  } catch (e) {
    console.error("\n  ❌", e.message);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

main();
