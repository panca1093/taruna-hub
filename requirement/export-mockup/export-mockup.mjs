/**
 * Tryout Center — Student Mockup Export
 *
 * Drives the running dev server (student role only) through the real
 * journey — landing, auth, catalog, checkout, dashboard, exam session,
 * result — screenshots each screen, and assembles them into one PDF.
 *
 * Prereq:
 *   cd ../web && npm run dev   (must be running on localhost:3411)
 *
 * Setup (one-time):
 *   cd export-mockup
 *   npm install
 *   npm run setup
 *
 * Run:
 *   npm run export:mockup
 *
 * Output:
 *   ./bimbel-tryout-mockup-siswa.pdf
 */

import { chromium } from "playwright";
import { writeFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const BASE = "http://localhost:3411";
const OUT_PDF = join(__dirname, "bimbel-tryout-mockup-siswa.pdf");
const W = 1440;
const H = 900;
const SCALE = 1.5;

// A product Naila (the demo student) does not already own, for the
// catalog → detail → checkout screens.
const DEMO_PRODUCT_ID = "prod-akpol-1";
// A tryout she already owns, for the dashboard → exam → result screens.
const DEMO_TRYOUT_ID = "tryout-sma-1";

// ─── Helpers ─────────────────────────────────────────────────────────────────

async function shot(page, screenshots, label) {
  await page.waitForTimeout(400);
  const buf = await page.screenshot({ fullPage: false });
  screenshots.push({ label, buf });
  console.log(`  ✓ ${label}`);
}

async function answerCurrentQuestion(page) {
  const box = page.locator('main div[class*="mt-6"][class*="space-y-2"]').first();
  const isian = box.locator('input[type="text"]');
  if ((await isian.count()) > 0) {
    await isian.fill("42");
    return;
  }
  await box.locator("button").first().click();
}

/** Answers every question in the current subtest and submits it. */
async function completeSubtest(page) {
  for (let guard = 0; guard < 30; guard++) {
    await answerCurrentQuestion(page);
    const nextBtn = page.getByRole("button", { name: "Selanjutnya" });
    if (await nextBtn.isDisabled()) break;
    await nextBtn.click();
    await page.waitForTimeout(150);
  }
  await page.getByRole("button", { name: "Submit Subtes" }).click();
  await page.waitForTimeout(400);
  await page.getByRole("button", { name: "Ya, Submit" }).click();
  await page.waitForTimeout(800);
}

// ─── Screen capture sequence ─────────────────────────────────────────────────

async function captureAll(page, screenshots) {
  // ── Public entry ─────────────────────────────────────────────────────────
  await page.goto(BASE + "/");
  await shot(page, screenshots, "01 — Beranda");

  await page.goto(BASE + "/login");
  await shot(page, screenshots, "02 — Masuk (Siswa)");

  await page.goto(BASE + "/register");
  await shot(page, screenshots, "03 — Daftar (Siswa)");

  // ── Catalog → purchase ───────────────────────────────────────────────────
  await page.goto(BASE + "/catalog");
  await shot(page, screenshots, "04 — Katalog");

  await page.goto(`${BASE}/catalog/${DEMO_PRODUCT_ID}`);
  await shot(page, screenshots, "05 — Detail Produk");

  await page.goto(`${BASE}/checkout/${DEMO_PRODUCT_ID}`);
  await shot(page, screenshots, "06 — Checkout");

  // ── Student dashboard ────────────────────────────────────────────────────
  await page.goto(BASE + "/student");
  await shot(page, screenshots, "07 — Tryout Saya");

  await page.goto(BASE + "/student/orders");
  await shot(page, screenshots, "08 — Riwayat Pesanan");

  await page.goto(BASE + "/student/profile");
  await shot(page, screenshots, "09 — Profil");

  // ── Exam-taking flow (real interaction, not seeded state) ───────────────
  await page.goto(`${BASE}/student/tryout/${DEMO_TRYOUT_ID}`);
  await shot(page, screenshots, "10 — Detail Tryout");

  await page.getByRole("link", { name: /Mulai|Lanjutkan/ }).click();
  await page.waitForTimeout(900);
  await shot(page, screenshots, "11 — Sesi Ujian");
  await completeSubtest(page);

  // Remaining subtests: complete without extra screenshots. The last
  // submit auto-redirects to the result page.
  for (let i = 0; i < 2; i++) {
    const startLink = page.getByRole("link", { name: /Mulai|Lanjutkan/ });
    if ((await startLink.count()) === 0) break;
    await startLink.click();
    await page.waitForTimeout(700);
    await completeSubtest(page);
  }

  await page.waitForTimeout(600);
  await shot(page, screenshots, "12 — Hasil Tryout");
}

// ─── PDF assembly ────────────────────────────────────────────────────────────

function buildPrintHTML(screenshots) {
  const pages = screenshots
    .map(({ label, buf }) => {
      const b64 = buf.toString("base64");
      return `<div class="pg">
      <div class="lbl">${label}</div>
      <img src="data:image/png;base64,${b64}" alt="${label}" />
    </div>`;
    })
    .join("\n");

  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8" />
<style>
  @page { size: ${W}px ${H}px; margin: 0; }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html, body { background: #fff; }
  .pg {
    width: ${W}px; height: ${H}px;
    page-break-after: always;
    break-after: page;
    position: relative;
    overflow: hidden;
  }
  .pg:last-child { page-break-after: avoid; break-after: avoid; }
  .lbl {
    position: absolute; top: 0; left: 0; right: 0;
    background: linear-gradient(90deg, #4f46e5, #8b5cf6);
    color: #fff;
    font: 600 11px/26px 'Helvetica Neue', sans-serif;
    padding: 0 12px; z-index: 10;
    letter-spacing: .02em;
  }
  img {
    width: ${W}px; height: ${H}px;
    object-fit: cover; object-position: top left;
    display: block;
  }
</style>
</head>
<body>${pages}</body>
</html>`;
}

// ─── Main ────────────────────────────────────────────────────────────────────

async function main() {
  console.log("\n▶  Tryout Center — Student Mockup Export");
  console.log(`   Target : ${BASE}`);
  console.log(`   Output : ${OUT_PDF}\n`);

  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    viewport: { width: W, height: H },
    deviceScaleFactor: SCALE,
  });
  await ctx.addInitScript(() => {
    window.localStorage.setItem("tryout-demo-role", "student");
    window.localStorage.setItem("theme", "light");
  });
  const page = await ctx.newPage();
  page.on("console", () => {});
  page.on("pageerror", (err) => console.warn("  ! page error:", err.message));

  const screenshots = [];

  try {
    await captureAll(page, screenshots);

    console.log(`\n  Captured ${screenshots.length} screens — assembling PDF…`);

    const html = buildPrintHTML(screenshots);
    await page.setContent(html, { waitUntil: "networkidle" });

    // Must match the @page CSS size exactly — page.pdf()'s width/height and
    // the stylesheet's @page size are two independent inputs to Chromium's
    // print pipeline; if they disagree, the content renders at the CSS size
    // and gets letterboxed inside the (larger) physical page. The captured
    // screenshots are still high-res (deviceScaleFactor above), so the
    // downscale into this CSS-pixel page stays crisp.
    const pdf = await page.pdf({
      width: `${W}px`,
      height: `${H}px`,
      printBackground: true,
      margin: { top: "0", right: "0", bottom: "0", left: "0" },
    });

    writeFileSync(OUT_PDF, pdf);
    console.log(`\n  ✅  PDF saved: ${OUT_PDF}`);
    console.log(`      Pages: ${screenshots.length}\n`);
  } catch (err) {
    console.error("\n  ❌ Error:", err.message);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

main();
