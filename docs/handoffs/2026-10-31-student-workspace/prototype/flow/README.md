# Student journey prototype, v3
## Latest palette decision

Owner memilih **rose hangat bersama untuk siswa dan admin** pada 31 Oktober 2026. Baca [PALETTE-DECISION.md](../../PALETTE-DECISION.md). Source/screenshot prototype tetap versi review sebelumnya: admin masih putih/biru; pilihan blue pada demo siswa hanya alternatif historis. Target handoff berikutnya adalah rose bersama, dengan sidebar plum, canvas krem dan state konsisten. Update dokumen ini tidak menerapkan CSS atau mengklaim visual rose admin sudah diverifikasi.



Portable entry (run from repository root):

```bash
python3 -m http.server 4181 --bind 127.0.0.1 --directory docs/handoffs/2026-10-31-student-workspace/prototype
```

Open http://localhost:4181/flow/#/dashboard. Read [HANDOFF.md](../../HANDOFF.md) before implementing: the real contract differs from some simulated payment/account flows.

This is a disposable visual prototype. All answers, credits, balances, orders and timers are synthetic and live only in a namespaced browser session. No API, SMTP, bank, payment gateway or real exam is called. Production code is untouched by this version. Indonesian is the language of the complete flow.

## Review paths

- Dashboard has equal prominence for normal tryouts and kecermatan.
- Tryout navigation opens a full-page hub with separate Tryout / Tes kecermatan tabs.
- Normal tryout: detail → readiness → per-subtest exam → full-page submission confirmation → next-subtest transition → result → answer explanations → history/retake.
- Kecermatan: detail/credits/history → readiness → A–E reference and missing-item drill → timed column → five-second transition → next column → counts/composites → per-question review by column and page.
- Catalogue → product detail → cart → checkout → payment instructions → pending/failed/review/confirmed → access or credits.
- Balance remains a quick popup as requested. Top-up and transaction history open full pages: amount → payment method → instructions → status → updated balance after confirmation.
- Profile, orders, preferences and email verification have full pages. The profile dropdown is navigation, not a content modal. Prototype options have their own page too.

## Demo controls

“Coba tampilan” at the page footer opens `/preview`. Reset to an active or a new account; try zero kecermatan credits; choose rose/blue and light/dark. Reset affects only this prototype's sessionStorage key.

Active sample account: two tryouts, four campur credits, Rp107,000 balance, two historical scores and one unpurchased symbol-test product in the cart. A new account starts empty. Use new account for a first-purchase walkthrough.

Regular tryout runtime is intentionally shortened to six sample questions, two per subtest, 45 seconds per subtest. It demonstrates single answer, true/false, text and multiple answers. Completion computes a demonstration score from those six examples; it is not the production grading implementation.

Kecermatan keeps ten columns and fifty questions per column, with the same hidden half-repetition structure as the existing application. The preview clock is accelerated to twenty seconds per column. References stay stable within a column; answer/skip is write-once and advances the question. It waits for the column deadline, then has a five-second pause with early continue. Credit decrements once on completion. Count totals and the current documented provisional composite calculation are represented for review, without changing D03.

“Percepat waktu” in the visibly labelled prototype strip lets a reviewer pass a subtest/column deadline. It is not a proposed student exam control. There is no real fullscreen activation or monitoring in this prototype.

Payment pages have visibly labelled simulation controls. Pending/failed orders grant nothing; a confirmed purchase grants access or adds credits once; a confirmed top-up adds balance once. Balance checkout checks funds and debits only that simulated balance. Manual payment can use the sample-proof control to demonstrate pending review before approval. File inputs do not upload anything to a server.

Cart is a proposed UI addition. The current application's HTTP checkout contract buys one product per order, so this concept allows storing several products but explicitly selecting one per payment. Combined multi-product payment would require a separate approved contract/product change. No coupon, tax or refund policy is invented.

## Prior prototype verification

- JavaScript syntax checks pass for `flow.js` and `data.js`.
- Browser exercised all six normal demo answers, all three locked subtest transitions, a score of 100, and six explanation rows.
- Browser exercised timed submission with blank answers and a real demonstration result of zero.
- Kecermatan answered/skipped, advanced across ten columns, produced a total of 500 correct/wrong/blank counts, decremented credit once, and exposed ten review rows per page with column selection.
- Cart kept a second item after paying the first; wallet checkout succeeded; online failure → retry → pending → success was exercised. Pending access was checked as absent before confirmation.
- Balance popup → top-up full page had zero open dialogs. Manual proof review → approval raised simulated balance from Rp82,000 to Rp132,000. Empty balance disabled balance checkout.
- Nine main pages passed 320/390/768/1024/1440/1920px horizontal-overflow checks (54 cases); normal exam and kecermatan runtime add six widths each (66 cases total). Mobile kecermatan was visually reviewed at 390px. A normal answer remained selected after reload and keyboard focus returned to the selected option after saving.
- Selected screenshots are under `../../reference/`. `layout-checks.json` records the browser geometry checks.

Limits: browser-only demo state; no claims of production auth, authoritative server deadlines, gateway integration, real upload persistence or a full accessibility audit. The older standalone v2 is not included in this portable bundle.
