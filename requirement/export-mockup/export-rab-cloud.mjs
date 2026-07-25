/**
 * Renders ../rab-cloud.md → ../rab-cloud.pdf using the same indigo→violet
 * template as export-docs.mjs (PRD_CSS). A4 landscape — the §3 provider
 * comparison table is 5 columns of long prose and overflows portrait.
 *
 * Run:   npm run export:rab-cloud
 */

import { chromium } from "playwright";
import { marked } from "marked";
import { readFileSync, writeFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const SRC = join(__dirname, "..", "rab-cloud.md");
const OUT = join(__dirname, "..", "rab-cloud.pdf");

// Same design system as export-docs.mjs PRD_CSS, with @page switched to A4
// landscape and margins widened to use the horizontal space.
const CSS = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

  @page { size: A4 landscape; margin: 15mm 18mm; }
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
  h1 + table {
    margin-top: 4px;
  }
  h2 {
    font-size: 10.5pt;
    color: #ffffff;
    background: linear-gradient(135deg, #4f46e5, #8b5cf6);
    margin: 22px 0 0;
    padding: 6px 11px;
    border-radius: 4px 4px 0 0;
    page-break-after: avoid;
  }
  h3 {
    font-size: 9.5pt;
    color: #1e1b3a;
    border-left: 4px solid #4f46e5;
    padding-left: 9px;
    margin: 18px 0 5px;
    page-break-after: avoid;
  }
  h4 {
    font-size: 9pt;
    color: #444;
    margin: 10px 0 3px;
  }

  p  { margin: 6px 0; }
  ul, ol { padding-left: 18px; margin: 4px 0; }
  li { margin: 3px 0; }
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

  em { color: #57606a; }
`;

// Explicit page-break points — exact heading text as marked emits it.
// §6 is intentionally NOT here so it flows onto the same page as §5.
const PAGE_BREAK_BEFORE = [
  "3. Perbandingan Provider — Rincian per Komponen (VM 2 vCPU / 4 GB RAM, setara e2-medium)",
  "5. Kenapa Biznet Gio Layak Dipertimbangkan",
];

function applyPageBreaks(html) {
  const pb = '<div style="break-before: page; page-break-before: always;"></div>';
  let result = html;
  for (const text of PAGE_BREAK_BEFORE) {
    const marker = `<h2>${text}</h2>`;
    if (!result.includes(marker)) {
      console.warn(`  ⚠ page-break marker not found for: ${text}`);
      continue;
    }
    result = result.replace(marker, pb + marker);
  }
  return result;
}

async function main() {
  console.log("\n▶  RAB Cloud — PDF export");
  console.log(`   Source : ${SRC}`);
  console.log(`   Output : ${OUT}\n`);

  const browser = await chromium.launch();
  const ctx = await browser.newContext({ viewport: { width: 1600, height: 900 } });
  const page = await ctx.newPage();
  page.on("console", () => {});

  try {
    const md = readFileSync(SRC, "utf8");
    const body = applyPageBreaks(marked.parse(md));

    const fullHtml = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8" />
<link rel="preconnect" href="https://fonts.googleapis.com" />
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
<style>${CSS}</style>
</head>
<body>${body}</body>
</html>`;

    await page.setContent(fullHtml, { waitUntil: "networkidle" });

    const pdf = await page.pdf({
      format: "A4",
      landscape: true,
      printBackground: true,
      margin: { top: "15mm", right: "18mm", bottom: "15mm", left: "18mm" },
    });
    writeFileSync(OUT, pdf);
    console.log(`  ✅  PDF saved: ${OUT}\n`);
  } catch (err) {
    console.error("\n  ❌ Error:", err.message);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

main();