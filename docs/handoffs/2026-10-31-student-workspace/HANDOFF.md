# Student workspace redesign — handoff

Tanggal audit: 31 Oktober 2026. Status: proposal prototype + panduan implementasi, bukan fitur production yang sudah selesai. Dokumen ini disiapkan atas permintaan owner untuk diteruskan ke agent lain.

## Mulai dari sini

1. Baca [prompt agent](START_HERE.md), dokumen ini, lalu [rencana kerja](PLAN.md).
2. Baca [keputusan rose hangat](PALETTE-DECISION.md), lalu jalankan prototype lokal dari bundle ini; tidak memerlukan folder visualizations milik pembuat. Source/screenshot lama belum semuanya mengikuti palet terpilih.
3. Cocokkan kembali branch, kontrak dan keputusan dengan checkout terbaru. Audit di bawah adalah snapshot, bukan jaminan bahwa main berikutnya sama.
4. Kerjakan satu paket yang diotorisasi owner, dengan bukti verifikasi; jangan mengubah policy untuk menyamakan aplikasi dengan simulasi.

```bash
python3 -m http.server 4181 --bind 127.0.0.1 --directory docs/handoffs/2026-10-31-student-workspace/prototype
```

Buka `http://localhost:4181/flow/#/dashboard`. Menu footer **Coba tampilan** membuka pengaturan demo: akun aktif/baru, kuota habis, rose/blue, light/dark. Semua jawaban, saldo, status pembayaran dan timer hanya simulasi browser. Tidak ada panggilan API, SMTP, bank atau gateway. Jangan gunakan akun atau bukti transaksi asli.

## Tambahan portal admin/operator

[Proposal desain admin/operator](ADMIN-PROPOSAL.md) ditambahkan setelah owner menanyakan portal admin. Dokumen terpisah ini mencakup navigasi berkelompok, wireframe dashboard operasional, flow monitoring/builder/transaksi dan audit gap API/mock. Statusnya **proposal + prototype lokal untuk review**, bukan bagian dari plan implementasi siswa. ZIP terbaru menyertakan source `prototype/admin/` dengan tiga journey operasional; buka `http://localhost:4182/admin/#/dashboard` atau ikuti README portabel. Belum ada perubahan aplikasi admin.

## Arah yang diminta owner

- Dashboard memenuhi lebar area di samping sidebar, dengan jarak komponen/modul yang konsisten. Dashboard sesudah login dan register memakai struktur yang sama; data dan empty state mengikuti akun.
- Navigasi utama cukup **Dashboard, Tryout, Katalog**. Sidebar bisa diciutkan; mobile memakai tiga tujuan yang sama.
- Profil, pesanan, saldo/riwayat transaksi, preferensi dan keluar diakses dari menu akun. Menu akun boleh dropdown, konten utama membuka halaman.
- Saldo cepat boleh popup. **Isi saldo wajib membuka halaman transaksi**, termasuk instruksi dan statusnya.
- Tryout normal dan Tes kecermatan adalah prioritas. Detail, persiapan, ujian, konfirmasi pengumpulan, transisi subtes, hasil, pembahasan dan riwayat harus menjadi flow yang bisa ditelusuri.
- Katalog → keranjang → checkout → pembayaran → status → akses/kuota. Yang dimaksud “card” adalah **cart/keranjang**.
- Pertahankan login siswa sebagai pintu publik; internal di `/portal`, tanpa tautan portal di depan. Verifikasi email enam digit boleh dilewati. Jangan tampilkan/log kode OTP.
- Animasi halus, tidak mengganggu ujian atau menyebabkan ilustrasi dan teks bertumpuk. Hormati reduced motion.

Owner menyukai arah visual dan flow. **Keputusan 31 Oktober 2026: gunakan rose hangat sebagai palet bersama siswa dan admin/operator**, termasuk auth terkait. Baca [keputusan palet dan pemetaan token](PALETTE-DECISION.md). Gate memilih rose vs biru sudah ditutup; preview admin biru dan screenshot lama belum menerapkan keputusan ini. Handoff tidak mengubah PRD, scoring, refund, checkout multiproduk atau policy akun. Jangan menganggap kontrol simulasi sebagai requirement produk.

## Snapshot checkout dan pekerjaan lokal

- Repo: `bimbel-tryout`; branch saat audit `codex/student-dashboard-full-width`.
- HEAD dan `origin/main` setelah fetch: `01c4a0203d1c15c4303a3c034217660514bf3d67` — katalog emoji/shape Kecermatan PR #31. Auth/dashboard PR #30 sudah ada di main.
- Sebelum handoff terdapat dua perubahan lokal belum di-commit: `web/app/student/page.tsx` menghapus batas `max-w-6xl` pada shell dashboard; `requirement/DESIGN.md` mencatat student full width dengan padding 16px mobile / 24px desktop.
- [full-width.patch](reference/full-width.patch) menyimpan diff tersebut terhadap HEAD di atas. Periksa `git diff` dahulu; jangan terapkan lagi jika perubahan sudah ada. Jangan reset, stash sembarang, atau kehilangan pekerjaan lokal.
- Dokumen/bundle ini belum di-commit atau di-push. Handoff tidak memberikan izin merge/deploy. Pertahankan author user, tanpa trailer atribusi agent.

## Apa yang sudah dicontohkan

| Flow | Yang bisa direview di prototype | Batas bukti |
|---|---|---|
| Dashboard | Akun aktif/baru, statistik, dua jenis latihan, aktivitas, empty state, menu akun | Angka contoh; bukan agregasi API |
| Tryout | Detail → kesiapan → 3 subtes → pengumpulan → hasil → pembahasan → riwayat | 6 soal, 45 detik/subtes; scoring lokal khusus demo |
| Kecermatan | Detail/kuota → kesiapan → kolom → jeda → hasil → review per kolom | 10 × 50 soal, 20 detik/kolom; referensi A–E contoh |
| Commerce | Katalog → detail → cart → checkout → pending/failure/review/success | Satu produk per pembayaran; status dipicu tombol simulasi |
| Wallet | Popup ringkas → topup full page → instruksi → status → ledger | Saldo dan proof tidak tersimpan di server |
| Akun | Profil, preferensi, pesanan, verifikasi sebagai halaman | Save/auth/verification belum membuktikan integrasi |

Screenshot acuan ada di [reference](reference/dashboard.jpg): [ujian normal](reference/tryout-exam.jpg), [hasil normal](reference/tryout-result.jpg), [review](reference/tryout-review.jpg), [Kecermatan](reference/kecermatan-exam.jpg), [mobile](reference/kecermatan-mobile.jpg), [hasil Kecermatan](reference/kecermatan-result.jpg), [cart](reference/cart.jpg), [checkout](reference/checkout.jpg), [saldo popup](reference/wallet-popup.jpg), [topup page](reference/topup-page.jpg), [pending](reference/payment-pending.jpg), [success](reference/payment-success.jpg). Screenshot bukan spesifikasi kontrak atau bukti transaksi.

## Flow yang kurang / harus ditutup saat implementasi

Sebagian besar backend sudah tersedia. Tabel ini membedakan pemulihan/perilaku yang harus dipertahankan dari kemampuan baru; jangan membangun ulang seluruh backend.

| ID / prioritas | Gap yang ditemukan | Perlakuan implementasi dan bukti selesai |
|---|---|---|
| G01 / P0 | Prototype belum membuktikan refresh, putus jaringan, deadline server, tab bersaing dan fullscreen tryout | Pakai runtime/hooks yang ada. Jawaban belum ACK terlihat belum tersimpan; reload melanjutkan sesi yang sama; waktu tetap berjalan; hasil hanya sekali. Entry/exit fullscreen dan pelaporan pelanggaran tetap berlaku. |
| G02 / P0 | Kecermatan demo hanya A–E dan satu konfigurasi; ACK gagal/terlambat belum lengkap | Render opsi dan aset dari API; uji konfigurasi riil termasuk image/emoji/shape/mix. Jawaban write-once, kolom kedaluwarsa tidak bisa diputar ulang. Uji retry habis/409, kuota nol, reload saat jeda dan completion oleh worker. |
| G03 / P0 | Normal result saat ini memilih completed attempt terbaru, bukan attempt yang diklik; held explanations belum diwakili penuh | Tambahkan pemilihan session eksplisit pada URL hasil normal, mengikuti pola Kecermatan `?attempt=<session_id>`. Riwayat lama harus membuka hasil lama. Hormati `explanations_released` dan `explanations_not_released`; jangan menampilkan kunci sebelum rilis. |
| G04 / P0 | Demo belum lengkap untuk bundle, sale window, harga berubah, akses sudah dimiliki dan pesanan pending | Re-fetch produk dan gunakan error/aturan API. Bundle boleh mengandung akses yang sudah dimiliki; server memberi yang belum ada. Konflik pending mengarahkan ke pesanan yang sama. |
| G05 / P0 | Urutan manual payment di demo berbeda dari kontrak | **Upload proof dahulu**, dapatkan `proof_asset_id`, baru create order/topup manual. Jangan membuat endpoint attach-proof rekaan. Uji upload gagal/tipe ditolak, ownership proof dan retry create. |
| G06 / P0 | Simulasi `failed/review/confirmed` menyederhanakan status uang riil | Pisahkan status order/topup dari attempt gateway. Reload/return/poll/cancel/expiry/unknown outcome harus menunjuk entity yang sama. Status final server saja yang memberi akses/saldo; held exception tidak membuka akses atau menjanjikan refund. |
| G07 / P0 | Topup belum membuktikan minimum dinamis, exact money, rejection dan duplicate submit | Gunakan `min_topup` dari server dan format exact numeric. Pending tidak menambah saldo, approved sekali, rejected menampilkan alasan. Isi saldo/status/ledger full page; tombol kembali tidak membuat transaksi baru. |
| G08 / P1 | Cart merupakan fitur UI baru; belum ada API cart | Draft per akun berisi product IDs, harga/availability diambil ulang. Beberapa item boleh disimpan, **satu produk dipilih per order**. Hapus item yang dibayar saja, isolasi logout/account switch. Multi-product payment menjadi scope/kontrak terpisah. |
| G09 / P1 | Profil aplikasi dulu mock: save/password hanya toast; belum ada endpoint self-update | Diputuskan owner 2026-10-31 dan dibangun di PR ini: siswa boleh mengubah nama dan telepon saja; email tetap dikelola admin; ganti password wajib password saat ini dan mencabut sesi lain milik user. Hanya role siswa untuk `PATCH /api/v1/me`. |
| G10 / P1 | Anonymous checkout → login/register → tujuan asal belum diperagakan | Audit guard yang ada dan tambah return intent internal yang diizinkan bila belum ada. Jangan menerima external redirect. Optional verify, sesi kedaluwarsa, recovery, dan `/portal` tetap benar. |
| G11 / P1 | Belum semua loading/error/partial/pagination/account switch terbukti | Jangan render nol palsu saat loading. Refresh wallet/access/dashboard setelah perubahan server, pertahankan nol skor yang sah, pisahkan data tiap akun. |
| G12 / P1 | Full flow masih ID-only; audit keyboard/contrast dan ultra-wide belum lengkap | ID/EN, light/dark, reduced motion, keyboard/focus/back, 320 sampai 2560px dan zoom. Shared shell admin tidak boleh kehilangan navigasi atau berubah visual tanpa scope. |

P0 di sini berarti prioritas penerimaan redesign pada area tersebut, bukan pernyataan bahwa semua sistem sekarang rusak. Flow sukses demo sudah ada; recovery dan bukti integrasi di atas adalah pekerjaan lanjut.

## Peta file dan route aplikasi

Seluruh path di tabel ini relatif terhadap root repo. Route baru adalah proposal implementasi, bukan endpoint yang sudah ada. Pertahankan URL lama atau sediakan kompatibilitas; hash route prototype tidak menjadi URL production.

| Area | File/route yang ada | Perubahan yang dituju |
|---|---|---|
| Shell/nav | `web/components/app-shell.tsx`, `web/lib/nav.ts`, `web/lib/session-context.tsx` | Shell siswa ringkas, menu akun, mobile3, full width; admin tetap kompatibel |
| Dashboard | `web/app/student/page.tsx` → `/student` | Layout sama untuk login/register; ringkasan API, normal/Kecermatan, state akun baru |
| Hub | `web/app/student/tryouts/page.tsx` → `/student/tryouts` | Dua tab jenis latihan, kepemilikan dan riwayat |
| Normal | `web/app/student/tryout/[id]/page.tsx`; `subtest/[subtestId]/page.tsx`; `result/page.tsx` | Detail/kesiapan/runtime/transisi; hasil `?attempt=` spesifik; reuse `web/lib/tryout-session.ts` |
| Kecermatan | `web/app/student/kecermatan/[id]/page.tsx`; `attempt/page.tsx`; `result/page.tsx` | Reuse runtime/API dan `web/components/kecermatan-item.tsx`; result sudah mendukung `?attempt=` |
| Katalog | `web/app/catalog/page.tsx`, `[id]/page.tsx` | Tetap anonymous/public; siswa signed-in mendapat shell baru |
| Cart (baru) | Usulan `web/app/student/cart/page.tsx`, `web/lib/student-cart.ts` | Draft IDs per akun, pilih satu produk lalu `/checkout/[id]` |
| Checkout | `web/app/checkout/[id]/page.tsx` → `/checkout/:productId` | Harga server, proof-before-create, wallet/manual/gateway |
| Pesanan | `web/app/student/orders/page.tsx` | List existing; usulan `orders/[id]/page.tsx` untuk status/detail pesanan yang bisa dibuka ulang |
| Wallet | `web/app/student/wallet/page.tsx` | Reuse ledger/topups; popup ringkas pada shell. Usulan `wallet/topup/page.tsx` dan `wallet/topups/[id]/page.tsx` untuk create/status |
| Akun | `web/app/student/profile/page.tsx`, auth/verify/recovery yang ada | Self-edit nama + telepon dan ganti password sudah dibangun di PR ini (keputusan owner 2026-10-31); email tetap readonly; preferensi reuse locale/theme existing |
| UI/i18n | `web/app/globals.css`, `web/lib/i18n/dict/`, `requirement/DESIGN.md` | Token/pola diperbarui bersama; semua copy ID+EN; reuse komponen existing |

## Kontrak yang wajib dipertahankan

### Sumber otoritatif dan stack

Baca `AGENTS.md`, `web/AGENTS.md`, `requirement/PRD.md`, `ARCHITECTURE.md`, `PLAN.md`, `RECONCILIATION.md`, `DESIGN.md`, `api/openapi.yaml`, lalu catatan domain `docs/`. PRD read-only. Ikuti panduan Next yang terpasang di `web/node_modules/next/dist/docs/` sebelum menulis Next code.

Tetap **Go 1.26 / Echo / pgx + Next.js 16 / React 19 / HeroUI v3 / Tailwind v4**. Prototype HTML/CSS/JS adalah referensi visual, bukan pengganti aplikasi. Semua data server lewat typed client `web/lib/api/`; jika kontrak perlu berubah, OpenAPI dahulu, generate schema, lalu backend/frontend. PostgreSQL otoritatif; jangan pindahkan scoring, uang, quota, auth atau deadlines ke browser.

### Auth dan dashboard

- `api.studentDashboard()` menyediakan ringkasan nyata; wallet berupa decimal string, nullable average tidak sama dengan skor nol. Jangan memakai seeded demo data atau mencampurkan metrik normal dengan Kecermatan tanpa label/sumber yang benar.
- Login/register memakai halaman dashboard yang sama. Kondisi unverified/empty berbeda berdasarkan klaim/data, bukan asal flow.
- Pertahankan role guard, sesi, safe redirect, optional verifikasi dan internal `/portal`. Endpoint/OTP/secret tidak masuk bundle atau log.

### Pembayaran dan wallet

- Katalog: `api.catalog.products(filters)`, `.product(id)`. Checkout: `api.orders.checkout(body)`, `.get(id)`, `.list(...)`, `.cancel(id)`, `.enrollments()`, `.bankTransfer()`, `.gateway()`.
- Topup: `api.topups.create(body)`, `.get(id)`, `.list(...)`, `.cancel(id)`; ledger `api.wallet.get()`, `.entries(...)`. Proof: `api.assets.uploadProof(file)`.
- `CheckoutBody`: gunakan schema generated, termasuk `product_id`, `expected_price`, `idempotency_key`, `payment_method` dan proof bila manual. Enum wire **`manual|wallet|gateway`**, bukan alias demo `saldo|online`.
- **Order.status = pending|confirmed|cancelled. Topup.status = pending|approved|rejected.** Status payment attempt berbeda. Provider deny/failure dapat meninggalkan order pending; jangan mengarang Order.status `failed` atau `review`.
- Uang tetap decimal string/exact numeric; gunakan `web/lib/format-money.ts`. Whole rupiah, minimum topup dari API (default setting Rp10.000), tidak mengarang batas maksimum.
- Create-timeout adalah hasil belum diketahui: pertahankan idempotency identity/payload, cari/lanjutkan entity yang sama. Retry tidak boleh menggandakan debit, credit, grant atau kuota. Replay key dengan payload berbeda ditolak server.
- Manual proof diunggah sebelum create; staged proof difinalisasi server dalam transaksi. Tidak ada POST bukti setelah create dalam flow yang ada. Jangan port urutan demo secara literal.
- Gateway availability dapat 503/unconfigured; jangan menawarkan metode yang tidak tersedia. Bank instructions juga harus tersedia sebelum manual create.
- Gunakan `redirect_url` yang ada untuk lanjut bayar; return URL bukan bukti lunas. Poll API status, cleanup saat unmount, sediakan refresh jika jaringan gagal.
- `payment_exception` (misalnya late_success, amount_mismatch, unexpected_state) mengikuti API. D07: late success setelah cancel/expiry ditahan untuk review staf, bukan auto-grant/refund. Jangan menawarkan staff approval untuk gateway-managed order.
- Topup manual/gateway saja; tidak membayar topup dari saldo wallet. Pending tidak menambah saldo. Rejection reason, cancel/expiry dan ledger reference harus bisa ditelusuri.
- P14 masih punya gate sandbox Midtrans nyata dan browser demo; desain baru tidak menutup gate itu hanya dengan fake provider. Layar admin held topup exception berada di luar scope redesign siswa.

### Tryout normal

- Gunakan `api.runtime.overview/start/session/subtest/startSubtest/saveAnswer/submitSubtest/attempts/result/review/reportViolation` sesuai typed signatures yang ada.
- Existing `useServerClock` memakai server_time dan monotonic clock. `useAnswerSync` mempertahankan pending/ACK, sequence, retry, unsent local recovery dan `stale_answer`; refactor tampilan tidak boleh merusaknya.
- Deadline = min(timer subtes, active_until) + grace + extension yang server tentukan. Browser hanya menampilkan waktu. Worker menutup sesi; reload tidak memulai sesi baru atau mengulang waktu.
- Subtes dimulai eksplisit; navigasi bebas hanya dalam subtes aktif, submit/expiry mengunci. Empat tipe soal, LaTex, gambar dan aset harus tetap berfungsi.
- Pertahankan fullscreen dan violation reporting P19. Larangan modal konten bukan izin menghapus guard/security overlay yang diperlukan runtime.
- `/student/tryout/[id]/result` sekarang mencari completed attempt terbaru. Gunakan session yang dipilih saat membuka history; legacy tanpa query tetap boleh memilih latest. API tetap mengotorisasi ownership; jangan percaya query client saja.
- Hasil dasar bisa tersedia saat pembahasan ditahan. Hormati release flag/time dan 409 `explanations_not_released`; kunci/pembahasan tidak boleh bocor. Ranking existing per tryout/average, tie 1,2,2,4; jangan menambah leaderboard nasional.

### Kecermatan

- Gunakan `api.kecermatanRuntime.tests/overview/start/session/next/answer/result/review`. History/result sudah mendukung selected attempt; jangan hilangkan.
- Konfigurasi riil dapat N genap 10–50, M 50–100, item length 1–5 dan opsi referensi dari API (dapat A–H). Jangan hardcode 10×50, A–E atau tiga karakter dari demo.
- Char, number, symbol, image, emoji, shape, mix tetap didukung; pertahankan katalog tambahan pada commit #31 dan renderer aset yang ada.
- Server menentukan identitas/repetisi, seed, phase kolom/pause/completed dan deadline. Jangan mengekspos struktur repetisi tersembunyi atau future keys ke siswa.
- Tidak ada early finish kolom. Jeda lima detik dapat dilanjutkan lebih cepat; bukan pause timer bebas. Kecermatan tidak memakai fullscreen/violation/leaderboard normal.
- Jawaban/skip write-once. Runtime existing punya retry terbatas dan menangani 409; audit retry habis dan ACK terlambat agar UI tidak memberi kepastian palsu atau menimpa jawaban server.
- Kuota diperlukan saat start dan dikurangi sekali saat selesai; tab tertutup dapat diselesaikan worker. Repeat purchase menambah enrollment yang sama, credits tidak kedaluwarsa. Jangan membuat cancel/refund kebijakan baru.
- D03 tetap terbuka: scoring versi 1 sementara, belum client-approved. Tampilkan output/count/version server; jangan menghitung ulang formula demo di browser atau mengklaim policy final.

## Gate keputusan dan batas scope

| Keputusan | Baseline yang bisa dikerjakan tanpa policy baru | Yang perlu keputusan terpisah |
|---|---|---|
| Palet — dipilih owner | Rose hangat bersama untuk auth, siswa dan admin; canvas, sidebar, kartu, teks, CTA dan state mengikuti [keputusan palet](PALETTE-DECISION.md) | Review hasil implementasi/contrast masih perlu; rebrand marketing dan seluruh fitur admin tidak otomatis masuk scope |
| Cart | Draft account-scoped, satu product/order, revalidasi saat checkout | Combined payment, quantities, coupon, refund policy |
| Profil | Nama + telepon dapat diubah siswa; email tetap dikelola admin; ganti password butuh password saat ini dan mencabut sesi lain | Diputuskan owner 2026-10-31 dan dibangun di PR ini; perubahan email/reverification tetap di luar scope |
| Kec scoring | Existing versioned output, label/rumus sesuai docs | Resolusi D03 oleh owner/client |
| Uang | Status/error dan exception existing; tidak auto-refund | Policy refund atau review baru dan admin held-exception tooling |

Gate yang terbuka tidak memblokir paket independen. Catat deferred secara jelas; jangan mengubah source of truth untuk mencocokkan prototype.

## Bukti yang sudah ada dan yang belum

Prototype sebelumnya diuji di browser: enam jawaban normal → skor demo 100, blank expiry → 0, jawaban tetap terpilih setelah reload; sepuluh kolom Kecermatan menghasilkan total 500 benar/salah/kosong dan decrement sekali; cart mempertahankan item kedua; wallet payment, online retry dan manual topup simulation; popup → topup page tanpa dialog tersisa. Geometry: 9 halaman × 6 widths + 2 runtimes × 6 = 66 pemeriksaan tanpa horizontal overflow (320/390/768/1024/1440/1920). Rekaman [layout-checks.json](prototype/flow/layout-checks.json) ikut dibawa.

Itu bukti prototype pada sesi review sebelumnya, **bukan** tes aplikasi real atau tes baru pada checkout agent berikutnya. Belum terbukti: gateway sandbox nyata, browser→API→DB seluruh flow, upload nyata, email provider, race/offline end-to-end, English full flow, 2560px, audit aksesibilitas/contrast lengkap. Jalankan ulang setelah implementasi.

## Acceptance akhir redesign

- [ ] Login dan register menuju struktur dashboard siswa yang sama; kosong/aktif/unverified ditentukan data server.
- [ ] Sidebar/mobile hanya tiga tujuan utama; akun membuka halaman terkait; saldo popup → isi saldo/status full page. Back/deep link/reload bekerja.
- [ ] Rose hangat diterapkan konsisten pada siswa dan shell admin sesuai PALETTE-DECISION; screenshot kedua area dibandingkan pada mode/viewport yang sama.
- [ ] Full width benar pada 1920/2560px; tidak ada wrapper max-width tersembunyi; normal/Kecermatan tetap terbaca pada 320px.
- [ ] Normal resume/offline/expiry/ACK/submit/locked-subtest/fullscreen serta selected historical attempt dan held review terbukti.
- [ ] Kecermatan opsi/aset riil, kuota nol, write-once, kolom/jeda/reload/worker completion/review dan kredit sekali terbukti.
- [ ] Catalog/bundle/cart revalidation, proof-before-create, wallet/manual/gateway, pending/unknown/cancel/held/reject dan idempotency memakai API; tidak ada simulation control di app.
- [ ] Saldo/grant/kuota berubah hanya sesudah server final; ledger/order/topup bisa ditelusuri, dashboard di-refresh tanpa data lintas akun.
- [ ] Profil tidak menampilkan sukses palsu; semua gate yang belum diputuskan disebut deferred.
- [ ] DESIGN §10: locale ID/EN, theme, keyboard, focus, reduced motion, contrast, zoom, loading/empty/error/partial; admin/public regression dicek.
- [ ] Checks repo dan demo real dilakukan sesuai rencana; hasil dicatat, bukan diasumsikan. PLAN diperbarui hanya untuk acceptance slice yang benar-benar dibuktikan.
