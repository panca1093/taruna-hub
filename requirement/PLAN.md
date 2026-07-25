# Delivery Plan — Platform Tryout Online (Taruna Eduka)

**Versi:** 1.0 · **Tanggal:** 2026-07-23 · **Basis:** PRD v1.2, RAB v1.3
**Target Go-Live:** 3 Agustus 2026

---

## 1. Cara Baca Dokumen Ini

Task dipecah **vertikal per fungsi & alur pengguna**, bukan horizontal per layer teknologi.
Artinya: tidak ada task "bikin semua tabel DB", "bikin semua endpoint", "bikin semua halaman".
Satu slice = satu kemampuan yang bisa dipakai orang, lengkap dari migration → endpoint → layar → integrasi → bisa didemo.

Setiap slice punya struktur sama:

| Bagian | Isi |
|---|---|
| **Outcome** | Satu kalimat: siapa bisa melakukan apa setelah slice ini selesai |
| **DB** | Migration yang dibutuhkan slice ini saja |
| **BE** | Endpoint + service Go untuk slice ini saja |
| **FE** | Halaman/komponen yang disentuh — mayoritas sudah ada sebagai mockup, tinggal disambung |
| **Integrasi** | Sambungan FE↔BE dan/atau pihak ketiga |
| **AC** | Kriteria selesai yang bisa diverifikasi, bukan "sudah jalan" |

**Aturan slice:** tidak ada slice yang dianggap selesai kalau hanya BE-nya jadi. Selesai = bisa didemo dari browser.

---

## 2. Titik Awal (per 2026-07-23)

| Bagian | Status | Catatan |
|---|---|---|
| `web/` mockup | **±85% UI** | 34 route, ~12.9k LOC, Next.js 16 + HeroUI. Seluruh data dari `lib/mock-data.ts` (in-memory) |
| Backend Go | **0%** | Belum ada `go.mod`, migration, atau service apapun |
| Integrasi pihak ketiga | **0%** | Midtrans, WhatsApp, email, object storage belum tersambung |
| Landing page | **Selesai (UI)** | Sudah pakai branding asli, tinggal dipisah deploy + disuapi data produk asli |

**Konsekuensi perencanaan:** kerjaan FE mayoritas adalah *mengganti sumber data*, bukan membangun layar dari nol.
Ini menurunkan risiko FE tapi memindahkan hampir seluruh bobot pekerjaan ke BE + integrasi.

### 2.1 Gap FE yang sudah terkonfirmasi dari audit kode

Delapan item ini bukan temuan baru untuk dibahas ulang — sudah dialokasikan ke slice-nya masing-masing di bawah:

| Gap | Slice |
|---|---|
| Randomisasi soal & opsi (FR-TRY-14/15, FR-ADM-TRY-06/07) — 0%, tidak ada di types maupun builder | S7, S8 |
| Checkout tidak persist apapun (`setSubmitted(true)` saja) | S5 |
| Window `active_from`/`active_until` tidak di-enforce | S8 |
| Export hasil CSV saja, PRD minta Excel/CSV | S9 |
| Tidak ada upload gambar di editor soal | S6 |
| Tombol edit user tanpa handler | S1 |
| Riwayat saldo belum punya ledger sendiri | S5 |
| `lib/types.ts` masih pisah `Tryout`/`KecermatanTest`, belum ikut unifikasi PRD §8 | S0 |

---

## 3. Peta Slice

```
MODUL RAB 1 — Fondasi & Auth (Rp 7.5jt, Minggu 1–2)
  S0  Walking Skeleton & Kontrak API          [enabler]
  S1  Akun & Sesi
  S2  Notifikasi (Email + WhatsApp)
  S3  Program & Struktur Materi
  S4  Landing Page Production

MODUL RAB 2 — Katalog & Transaksi (Rp 13.5jt, Minggu 1–2)
  S5  Produk & Katalog
  S6  Checkout, Pembayaran & Enrollment

MODUL RAB 3 — Tryout Engine (Rp 28.5jt, Minggu 3–4)
  S7  Bank Soal
  S8  Tryout Builder
  S9  Pengerjaan Tryout & Proctoring        ← risiko tertinggi
  S10 Hasil, Scoring & Ranking
  S11 Live Monitoring

MODUL RAB 4 — Tes Kecermatan (Rp 7.5jt, Minggu 3–4)
  S12 Kecermatan End-to-End

MINGGU 5 — UAT & Go-Live
  S13 Hardening, Load Test & Rilis
```

### Dependency

```
S0 ──┬─> S1 ──┬─> S2
     │        ├─> S3 ──> S7 ──> S8 ──> S9 ──> S10 ──> S11
     │        └─> S5 ──> S6 ──┬─> S9
     │                        └─> S12
     └─> S4
S13 <── semua
```

Jalur kritis: **S0 → S1 → S3 → S7 → S8 → S9 → S10**. Slice di jalur ini tidak bisa diparalelkan tanpa menambah orang.

---

## 4. Slice

### S0 — Walking Skeleton & Kontrak API
**Modul 1** · **Depends:** — · **Size:** M · **FR:** — (enabler)

> **Outcome:** Satu request nyata mengalir dari browser → Echo → Postgres → balik ke layar, dan seluruh slice berikutnya punya pola yang tinggal diikuti.

Ini satu-satunya slice yang horizontal, dan sengaja dibuat setipis mungkin: ia membuktikan diri lewat satu endpoint asli (`/healthz` + `GET /me`), bukan lewat kerangka kosong.

- **DB**
  - Scaffold `golang-migrate`, konvensi penamaan migration, database lokal via Docker Compose (Postgres + Redis + MinIO)
  - Migration `0001_init`: `users`, `programs` (minimal, cukup untuk auth + smoke test)
- **BE**
  - Repo Go: layout `model → repository → service → handler/server` + `infra`, `adapter` (sesuai PRD §7)
  - Echo v4 + pgx/v5 pool, config loader, structured logging, request ID, panic recovery
  - Error envelope seragam (kode + pesan) supaya FE tidak perlu menebak bentuk error per endpoint
  - `GET /healthz`, `GET /api/v1/me`
- **FE**
  - **Selaraskan `lib/types.ts` dengan PRD §8**: satukan `Tryout`+`KecermatanTest` → `Test`, `Enrollment`+`KecermatanEnrollment` → `Enrollment`, `TryoutSession` → `TestSession`. Dikerjakan sekarang selagi konsumen tipe masih sedikit — kalau ditunda, sentuhannya melebar ke seluruh slice
  - Buat seam `lib/api/` sebagai satu-satunya pintu data; `lib/mock-data.ts` dipindah ke belakang seam ini supaya bisa dicabut per-domain, bukan sekaligus
- **Integrasi**
  - Kontrak OpenAPI 3.0 sebagai sumber kebenaran; generate typed client TS untuk FE
  - `.env` + config untuk dev/staging/prod
  - CI: `go build`, `go vet`, `go test`, `next build`, lint
- **AC**
  - `docker compose up` → stack lokal jalan dari nol tanpa langkah manual
  - Halaman FE memanggil `GET /api/v1/me` lewat typed client dan menampilkan hasil asli dari Postgres
  - `go test ./...` dan `next build` hijau di CI

---

### S1 — Akun & Sesi
**Modul 1** · **Depends:** S0, **S2 (pengiriman email OTP)** · **Size:** M→L · **FR:** FR-AUTH-01…05

> **Outcome:** Siswa bisa daftar & masuk pakai email atau nomor HP; admin bisa mengelola akun siswa dan operator dari dashboard.

> **Perubahan scope dari PRD (2026-07-23):** PRD §7 menyatakan "JWT (tanpa OTP — tidak diperlukan untuk skala ini)". Klien/owner memutuskan **OTP tetap dipakai**, dikirim via email lebih dulu (WhatsApp menyusul setelah provider ditetapkan). Ini menambah S1 ±1 langkah dan membuat S1 bergantung pada S2, yang tadinya sebaliknya — lihat catatan dependency di bawah.

- **DB**
  - Lengkapi `users`: `phone` unik-nullable, `password_hash`, `role`, `balance`, `verified_at`
  - `password_reset_tokens` (token hash, expiry, single-use)
  - `otp_codes` — kode hash, tujuan (`register`/`login`/`reset`), expiry, jumlah percobaan, `consumed_at`
- **BE**
  - `POST /auth/register`, `POST /auth/login` (identifier = email **atau** phone), `POST /auth/refresh`, `POST /auth/logout`
  - `POST /auth/forgot-password`, `POST /auth/reset-password`
  - **OTP:** `POST /auth/otp/request`, `POST /auth/otp/verify` — kode disimpan ter-hash (bukan plaintext), sekali pakai, expiry pendek, cooldown kirim ulang, kunci setelah N percobaan gagal
  - bcrypt, JWT access + refresh, auto-logout saat idle
  - Middleware role guard: `student` / `operator` / `admin`
  - `GET/POST/PATCH/DELETE /admin/users` — admin-only
- **FE**
  - Sambungkan `/login`, `/login/staff`, `/register`, `/forgot-password` ke endpoint asli
  - `lib/session-context.tsx`: ganti user mock dengan sesi JWT nyata + refresh diam-diam
  - `components/require-role.tsx`: guard berbasis klaim token, bukan role mock
  - **Perbaiki tombol edit user di `app/admin/users/page.tsx`** (saat ini tanpa handler)
- **Integrasi**
  - Cabut `currentStudent` / `currentOperator` / `currentAdmin` dari mock-data
  - Interceptor 401 → refresh sekali → kalau gagal, lempar ke `/login`
- **AC**
  - Daftar pakai nomor HP → login pakai nomor HP yang sama → berhasil
  - Operator membuka URL admin-only (`/admin/programs`, `/admin/users`) → ditolak
  - Reset password: token sekali pakai, dan tidak bisa dipakai lagi setelah dikonsumsi
  - Admin bisa membuat, **mengedit**, dan menghapus akun operator maupun siswa
  - OTP: kode kedaluwarsa tepat waktu, tidak bisa dipakai dua kali, dan brute-force 6-digit terkunci setelah N percobaan
  - Kode OTP **tidak pernah** muncul di log aplikasi

---

### S2 — Notifikasi (Email + WhatsApp)
**Modul 1** · **Depends:** S1 · **Size:** M · **FR:** FR-NOT-01…04

> **Outcome:** Event penting (pesanan, akses terbuka, reset password) mengirim email + WhatsApp, dan tetap terkirim walau proses sempat mati.

Dibangun lebih awal daripada pemakainya, supaya slice S6/S9/S12 tinggal `Publish(event)` tanpa menyentuh infrastruktur lagi.

- **DB**
  - Schema `river` terpisah dari schema bisnis (PRD §7.1)
  - `notification_logs` — audit siapa dikirimi apa, kapan, status akhir
- **BE**
  - In-process event bus pola Mediator: `Publish(event)` → handler terdaftar
  - **Event kritikal**: handler hanya insert job River **di transaction yang sama** dengan perubahan state bisnis — job tercatat kalau dan hanya kalau transaction commit
  - **Event non-kritikal**: jalan in-memory, boleh hilang
  - River worker: retry + backoff, retention cleanup job `completed`/`discarded`
  - Template email: konfirmasi pesanan, akses terbuka, reset password, **kode OTP**
- **FE**
  - (tidak ada layar baru)
- **Integrasi**
  - **Pengirim email — diputuskan 2026-07-23:** pakai akun email milik owner (`panggilsaya.saiful@gmail.com`) lebih dulu untuk dev & UAT, supaya S1/S6 tidak menunggu akses DNS domain klien.
    **Batasannya harus disadari dan dijadwalkan penggantiannya:** Gmail membatasi ±500 kirim/hari, dan email transaksional dari domain gmail.com tanpa SPF/DKIM atas nama Taruna Eduka rawan masuk spam — tepat pada email yang paling tidak boleh hilang (OTP & konfirmasi pembayaran). **Cukup untuk dev/UAT, tidak cukup untuk go-live.** Migrasi ke domain klien + provider transaksional masuk S13 sebagai syarat rilis, bukan opsional
  - Interface `adapter/notifier` dari awal, supaya penggantian pengirim nanti hanya mengganti satu implementasi
  - **Provider WhatsApp — belum ditentukan, lihat §6 Keputusan Terbuka #1.** Sampai diputuskan, implementasi no-op di balik interface yang sama supaya slice lain tidak terblokir
- **AC**
  - Matikan worker → picu event kritikal → nyalakan worker → notifikasi tetap terkirim
  - Rollback transaction bisnis → job notifikasi ikut tidak ada (buktikan dengan test)
  - Job gagal 3× masuk `discarded` dan tercatat di `notification_logs`, tidak retry selamanya

---

### S3 — Program & Struktur Materi
**Modul 1** · **Depends:** S1 · **Size:** S · **FR:** FR-ADM-PRG-01, FR-ADM-PRG-02

> **Outcome:** Admin menyusun Program → Mata Pelajaran → Bab Materi, yang jadi sumber dropdown saat membuat soal.

- **DB** — `programs`, `program_subjects`, `subject_chapters`
- **BE** — CRUD bertingkat, **admin-only** (Operator ditolak — perubahan dari v1.0); guard hapus kalau program masih dipakai soal/tryout/produk
- **FE** — `app/admin/programs/page.tsx` disambung ke API asli
- **Integrasi** — cabut `programs` dari mock-data; dropdown di editor soal (S7) baca dari sini
- **AC**
  - Operator kena 403 di seluruh endpoint program
  - Hapus program yang masih punya soal → ditolak dengan pesan jelas, bukan error 500
  - Bab materi muncul di dropdown editor soal setelah dibuat, tanpa reload manual

---

### S4 — Landing Page Production
**Modul 1** · **Depends:** S0 · **Size:** S · **FR:** FR-LAND-01…06

> **Outcome:** Halaman promosi tayang di domain utama dengan produk unggulan yang asli, bukan hardcoded.

UI-nya sudah jadi. Kerjaannya adalah pemisahan deploy + sumber data.

- **BE** — `GET /public/products?featured=true` (tanpa auth, cache-able)
- **FE** — pisahkan build/deploy landing dari aplikasi tryout; produk unggulan & jumlah program tarik dari API
- **Integrasi** — pemetaan domain (lihat §6 Keputusan Terbuka #2); CTA "Lihat Katalog" mengarah lintas-domain dengan benar
- **AC**
  - Produk unggulan berubah di landing setelah admin menonaktifkan produk, tanpa deploy ulang
  - Lighthouse mobile ≥ 90 untuk Performance & Accessibility
  - Tautan portal internal (Operator/Admin) di footer mengarah ke host aplikasi yang benar

---

### S5 — Produk & Katalog
**Modul 2** · **Depends:** S1, S3 · **Size:** M · **FR:** FR-CAT-01…05, FR-ADM-PRD-01…03

> **Outcome:** Operator membuat produk (individual/bundle/kecermatan) dan produk itu langsung muncul di katalog publik untuk dilihat siswa.

Sengaja satu slice: "produk dibuat" tanpa "produk terlihat" bukan kemampuan yang bisa didemo.

- **DB** — `products`, `product_tests` (junction tunggal hasil unifikasi PRD §8, termasuk `included_attempts` untuk tipe kecermatan)
- **BE**
  - Publik: `GET /catalog` (filter per program, hanya `is_active`), `GET /catalog/:id`
  - Admin: CRUD produk, toggle aktif/nonaktif
  - Validasi per tipe: `individual` = tepat 1 test; `bundle` = N test; `kecermatan` = 1 test + `included_attempts` wajib
- **FE** — `app/catalog`, `app/catalog/[id]`, `app/admin/products` disambung ke API
- **Integrasi** — cabut `products` dari mock-data; upload cover image ke object storage (MinIO lokal / GCS prod)
- **AC**
  - Produk nonaktif tidak muncul di katalog publik, tapi tetap terlihat di dashboard admin
  - Filter program mengembalikan hanya produk program tersebut
  - Produk bundle menampilkan seluruh tryout anggotanya di halaman detail
  - Bikin produk kecermatan tanpa mengisi `included_attempts` → ditolak validasi

---

### S6 — Checkout, Pembayaran & Enrollment
**Modul 2** · **Depends:** S2, S5 · **Size:** L · **FR:** FR-PAY-A-01…03, FR-PAY-C-01…04, FR-PAY-W-01…03, FR-PAY-01/02, FR-ADM-ORD-01…03

> **Outcome:** Siswa memilih salah satu dari 3 metode pembayaran, membayar, dan aksesnya terbuka — otomatis (gateway/saldo) atau setelah admin konfirmasi (manual).

Slice terberat di Modul 2, dan gap FE terbesar saat ini: checkout sekarang murni tampilan, tidak menyimpan apapun.

- **DB**
  - `orders` (status, `payment_mode`, `payment_proof_url`, `confirmed_at`)
  - `enrollments` — generik: `active_until` **dan** `attempts_purchased`/`attempts_used` sama-sama nullable, dipakai sesuai tipe produk
  - `wallet_transactions` — ledger `topup`/`deduction`, sumber kebenaran saldo
- **BE**
  - `POST /orders` — satu endpoint, tiga cabang `payment_mode`
  - **Gateway:** buat transaksi Midtrans → simpan `snap_token`; `POST /webhooks/midtrans` verifikasi signature, **idempotent** (webhook boleh datang berkali-kali), `confirmed` → terbitkan enrollment
  - **Manual:** order `pending` → siswa unggah bukti → admin konfirmasi/batalkan
  - **Saldo:** potong saldo & terbitkan enrollment **dalam satu transaction**; tolak kalau saldo kurang. Saldo dihitung dari ledger, bukan kolom yang di-update terpisah
  - Penerbitan enrollment terpusat di satu service — dipanggil ketiga jalur, supaya aturan akses tidak bercabang tiga
  - Admin: daftar pesanan + filter status, konfirmasi, batalkan, lihat bukti transfer, top-up saldo siswa (**admin & operator**)
  - Publish event kritikal ke S2 pada `confirmed`
- **FE**
  - **Tulis ulang `app/checkout/[id]/page.tsx`** supaya benar-benar membuat order (saat ini hanya `setSubmitted(true)`)
  - Redirect Midtrans Snap + halaman status kembali
  - Unggah bukti transfer di `app/student/orders`
  - **Halaman riwayat saldo** dari `wallet_transactions` (saat ini top-up hanya menumpang di daftar pesanan)
  - `app/admin/orders` disambung ke API
- **Integrasi**
  - Midtrans sandbox → produksi: server key, verifikasi signature, penanganan webhook telat/duplikat
  - Object storage untuk bukti transfer: validasi tipe & ukuran (maks 5MB)
- **AC**
  - Tiga metode masing-masing menghasilkan enrollment yang **identik bentuknya** — sisanya hanya beda jalur
  - Kirim webhook Midtrans yang sama 3× → tepat 1 enrollment, saldo/status tidak berubah di kirim ke-2 dan ke-3
  - Bayar pakai saldo saat saldo kurang → ditolak, dan **tidak ada** baris ledger yang tercipta
  - Simulasikan gagal di tengah pembayaran saldo → saldo utuh, tidak ada enrollment yatim
  - Admin membatalkan pesanan pending → akses tidak pernah terbuka
  - Setelah `confirmed`, siswa melihat paketnya di "Paket Belajar Saya" dan menerima email + WhatsApp

---

### S7 — Bank Soal
**Modul 3** · **Depends:** S3 · **Size:** L · **FR:** FR-ADM-SOA-01…06, §5.5, §5.6 (bobot per opsi)

> **Outcome:** Operator membuat soal 4 tipe lengkap dengan LaTeX dan gambar, atau mengunggah ratusan soal sekaligus dari Excel/Word.

- **DB** — `questions` dengan `options jsonb` (tiap opsi: id, label, points nullable), `correct_answer jsonb`, `option_weighted bool`
- **BE**
  - CRUD soal + filter program/tipe/kesulitan + paginasi
  - Validasi per tipe: `pg` 1 benar; `pg_multi` >1 benar; `isian` pola pencocokan; `benar_salah` biner
  - Mode **bobot per opsi**: seluruh opsi valid, opsi tanpa bobot eksplisit default **1**
  - Import massal `.xlsx` / `.csv` / `.docx`: parse → validasi baris-per-baris → laporan galat → commit atomik
  - Endpoint unduh template
- **FE**
  - `components/admin/question-editor.tsx` disambung ke API
  - **Tambahkan upload gambar** (`imageUrl` sudah ada di tipe tapi tidak terjangkau dari UI)
  - **Ganti preview import yang masih hardcoded** dengan hasil parse berkas yang benar-benar diunggah
  - Tampilkan ID soal di daftar (FR-ADM-SOA-06)
- **Integrasi** — object storage untuk gambar soal (validasi tipe + maks 5MB); render LaTeX via KaTeX (sudah terpasang)
- **AC**
  - Impor 200 soal: baris valid masuk, baris rusak dilaporkan per-nomor-baris, dan **tidak ada impor separuh jadi**
  - Soal bermode bobot-per-opsi menyimpan bobot berbeda per opsi, dan opsi yang dikosongkan tersimpan bernilai 1
  - Preview menampilkan LaTeX & gambar persis seperti yang nanti dilihat siswa
  - Filter kombinasi (program + tipe + kesulitan) mengembalikan hasil yang benar

---

### S8 — Tryout Builder
**Modul 3** · **Depends:** S7 · **Size:** M · **FR:** FR-ADM-TRY-01…07

> **Outcome:** Operator merakit tryout: subtes, durasi, soal, jendela tanggal, penahanan pembahasan, dan toggle pengacakan.

- **DB**
  - `tests` (base: program, tipe, judul, deskripsi, cover) + `tryout_details` (`scoring_mode`, `active_from`, `active_until`, `explanation_release_at`, **`randomize_questions`**, **`randomize_options`**)
  - `subtests`, `subtest_questions`
- **BE**
  - CRUD tryout + subtes + assign soal berurutan
  - **Edit-lock:** tolak ubah/hapus kalau sudah ada session — berlaku untuk **seluruh** field, bukan sebagian
  - Validasi: `active_from` < `active_until`; tryout tanpa subtes tidak bisa diterbitkan
- **FE**
  - `components/admin/tryout-builder.tsx` disambung ke API
  - **Tambahkan toggle "Acak Urutan Soal" & "Acak Urutan Jawaban"** — saat ini tidak ada di builder maupun di `lib/types.ts`
  - Upload cover image (sekarang masih pilihan gradien)
- **Integrasi** — `components/admin/question-picker-drawer.tsx` menarik dari bank soal asli
- **AC**
  - Toggle pengacakan tersimpan dan terbaca ulang setelah refresh
  - Tryout yang sudah dikerjakan siswa: seluruh field terkunci, bukan hanya sebagian
  - Urutan soal dalam subtes tersimpan sesuai susunan operator

---

### S9 — Pengerjaan Tryout & Proctoring
**Modul 3** · **Depends:** S6, S8 · **Size:** XL · **FR:** FR-TRY-01…15

> **Outcome:** Siswa mengerjakan tryout dengan timer, tidak bisa mundur ke subtes yang sudah dikirim, bisa lanjut setelah browser tertutup, dan pelanggaran fullscreen/screenshot tercatat.

**Slice paling berisiko di seluruh proyek.** Semua hal yang "diam-diam salah" ada di sini: timer, kehilangan koneksi, dan pengacakan. Anggarkan buffer di sini, bukan di slice lain.

- **DB** — `test_sessions` (`attempt_number`, `status`, `progress jsonb`, UNIQUE(user, test, attempt)), `tryout_answers`, `violations`
- **BE**
  - `POST /tests/:id/sessions` — mulai attempt; **enforce jendela `active_from`/`active_until`** (gap FE saat ini) dan enrollment aktif
  - **Timer otoritatif di server.** Waktu sisa dihitung dari `started_at` di DB, jam klien hanya untuk tampilan — kalau tidak, timer bisa dicurangi dengan mengubah jam perangkat
  - Auto-submit saat timer habis, toleransi ±5 detik (NFR §6.1)
  - Blocking time: subtes terkirim → ditolak permanen kalau dibuka lagi
  - Simpan jawaban inkremental supaya resume tidak kehilangan progres
  - **Pengacakan (FR-TRY-14/15):** urutan diacak **sekali** saat session dibuat, disimpan di `progress jsonb`, lalu dipakai konsisten sepanjang session — tidak berubah saat siswa bolak-balik antar soal. Bobot opsi melekat pada **konten** opsi, bukan posisi tampil
  - `POST /sessions/:id/violations` — `exit_fullscreen`, `screenshot_attempt`
- **FE**
  - `app/student/tryout/[id]/subtest/[subtestId]` disambung ke API; timer sinkron ke server, bukan hitung mundur lokal murni
  - Terapkan urutan acak dari server, jangan mengacak ulang di klien
  - Fullscreen + deteksi screenshot sudah ada — arahkan pencatatannya ke API
  - Perilaku offline: antre jawaban, kirim ulang saat koneksi kembali
- **Integrasi** — Redis cache konten subtes pre-shuffle per `subtest_id`; API konten ujian **`Cache-Control: no-store`** (PRD §7.2 — kalau ikut ter-cache CDN, semua siswa dapat urutan acak yang sama dan tujuan pengacakan batal)
- **AC**
  - Tutup browser di tengah subtes → buka lagi → **waktu tersisa sudah berkurang sesuai waktu nyata**, jawaban utuh
  - Ubah jam sistem perangkat → waktu sisa tidak terpengaruh
  - Timer habis saat tab tidak aktif → subtes tetap auto-submit
  - Subtes terkirim → tidak bisa dibuka lagi lewat URL langsung
  - Dengan `randomize_questions` aktif: dua attempt berbeda → urutan berbeda; **dalam satu attempt, navigasi bolak-balik → urutan tetap**
  - Dengan `randomize_options` aktif: skor soal bobot-per-opsi identik terlepas dari posisi tampil opsi
  - Mulai tryout di luar jendela tanggal → ditolak
  - Keluar dari fullscreen → tercatat dengan timestamp, overlay peringatan muncul, **timer terus berjalan**

---

### S10 — Hasil, Scoring & Ranking
**Modul 3** · **Depends:** S9 · **Size:** L · **FR:** FR-RES-01…05, §5.6, FR-ADM-RES-01…04

> **Outcome:** Siswa melihat skor, peringkat, dan pembahasan; admin melihat & mengekspor hasil seluruh siswa.

- **DB** — `tryout_results` (`total_score`, `subtest_scores jsonb`, `rank`, `irt_warning`)
- **BE**
  - Mesin skoring: weighted per kesulitan (konfigurasi global), bobot-per-opsi, mode IRT
  - **IRT prevalidation:** soal dengan respons < 30 siswa → `irt_warning = true`, tampil di dashboard admin
  - **Ranking = rata-rata skor seluruh attempt siswa** pada tryout tersebut (FR-RES-05), bukan skor satu attempt
  - Gating pembahasan: sebelum `explanation_release_at`, kembalikan skor **tanpa** konten pembahasan — disaring di server, bukan disembunyikan di FE
  - Export hasil
- **FE**
  - `app/student/tryout/[id]/result` + riwayat seluruh attempt
  - `app/admin/results` disambung ke API
  - **Tambahkan export Excel** — saat ini CSV saja, PRD minta Excel/CSV
- **Integrasi** — Redis Sorted Set untuk leaderboard; generate export sebagai job River (S2) supaya tidak memblokir request
- **AC**
  - Siswa mengerjakan 3×, skor 60/80/100 → peringkat memakai rata-rata **80**, bukan 100
  - Sebelum tanggal rilis pembahasan: **response API tidak memuat konten pembahasan sama sekali** (cek payload mentah, bukan tampilan)
  - Tryout dengan soal <30 respons → penanda IRT warning muncul di dashboard admin
  - Berkas Excel hasil export terbuka benar di Excel & Google Sheets, dengan angka sebagai angka
  - Skor bobot-per-opsi cocok dengan hitungan manual pada satu kasus uji

---

### S11 — Live Monitoring
**Modul 3** · **Depends:** S9 · **Size:** S · **FR:** FR-MON-01…04

> **Outcome:** Admin & operator melihat siapa yang sedang ujian sekarang dan pelanggaran apa yang terjadi.

- **BE** — `GET /admin/monitoring/sessions` (siswa aktif, tryout, subtes berjalan, waktu mulai, jumlah pelanggaran); `GET /admin/sessions/:id/violations` timeline
- **FE** — `app/admin/monitoring` disambung ke API, polling 30 detik (tanpa WebSocket per PRD §5.16); timeline pelanggaran juga tersedia dari halaman hasil untuk sesi selesai
- **Integrasi** — event non-kritikal (in-memory, boleh hilang — PRD §7.1); query dibatasi supaya polling tidak membebani DB
- **AC**
  - Siswa memulai subtes → muncul di monitoring dalam ≤30 detik
  - Siswa menyelesaikan tryout → hilang dari daftar aktif, tapi violation log-nya tetap bisa dibuka dari halaman hasil
  - 50 sesi aktif bersamaan → satu putaran polling tetap p95 ≤500ms

---

### S12 — Kecermatan End-to-End
**Modul 4** · **Depends:** S6 · **Size:** L · **FR:** FR-KEC-01…14, FR-ADM-KEC-01…05

> **Outcome:** Operator membuat tes kecermatan & produknya; siswa membeli sejumlah attempt, mengerjakan, dan melihat 3 skor komposit + Nilai Akhir.

Satu slice utuh karena format ini punya produk, billing, engine, dan hasilnya sendiri yang terpisah dari tryout. UI-nya sudah dibangun di mockup (2026-07-20) — kerjaannya BE + integrasi.

- **DB** — `kecermatan_details`, `kecermatan_answers`, `kecermatan_results`
- **BE**
  - Builder: tipe item, panjang item, opsi per soal, soal per kolom, jumlah kolom, durasi timer per kolom; edit/hapus terkunci kalau sudah dikerjakan
  - Engine: **generate set referensi per kolom di server** (bukan di klien — kalau di klien, jawabannya ada di memori browser), subset acak dengan tepat 1 item hilang per soal, timer bersama per kolom, soal tak terjawab saat timer habis = `kosong` (bukan salah), jeda transisi antar kolom
  - **Billing metered:** attempt memotong kuota; kuota habis → arahkan beli tambahan yang **menambah kuota pada enrollment yang sama**, bukan transaksi terpisah
  - Skor komposit Kecepatan / Ketahanan / Ketelitian + Nilai Akhir tertimbang (30/45/25)
  - Tanpa fullscreen, tanpa violation log, tanpa ranking (FR-KEC-06 — sengaja berbeda dari tryout)
- **FE** — halaman kecermatan siswa & admin disambung ke API; hentikan generate soal di klien
- **Integrasi** — pakai jalur checkout S6 apa adanya; hanya penerbitan enrollment yang bercabang (attempt, bukan tanggal)
- **AC**
  - Kuota habis → tombol mulai terkunci dan siswa diarahkan ke checkout
  - Beli attempt tambahan → kuota **bertambah pada enrollment yang sama**, riwayat attempt lama tetap utuh
  - Timer kolom habis → sisa soal tercatat `kosong`, dan `kosong` **tidak** dihitung sebagai salah di Ketelitian
  - Set referensi tidak pernah muncul di response API sebelum soalnya ditampilkan
  - Skor komposit cocok dengan contoh perhitungan klien (2026-07-05) pada dataset yang sama
  - **Formula ditandai provisional di UI** sampai klien konfirmasi (§6 Keputusan Terbuka #3)

---

### S13 — Hardening, Load Test & Rilis
**Minggu 5** · **Depends:** semua · **Size:** M · **FR:** §6 NFR

> **Outcome:** Platform tayang produksi dan bertahan di 1.000 CCU.

- **BE** — rate limiting endpoint auth, **request OTP**, & submit jawaban; audit query N+1 di jalur ujian; index sesuai pola query nyata
- **Email produksi** — pindah dari akun Gmail owner ke domain klien + provider transaksional, SPF/DKIM/DMARC terpasang, uji deliverability OTP & konfirmasi pembayaran ke Gmail/Outlook. **Syarat rilis, bukan opsional** (lihat S2)
- **Infra** — HTTPS, Cloudflare CDN (aset statis saja), backup harian retensi 30 hari + **uji restore**, health check, alerting
- **Test** — load test 1.000 CCU terkonsentrasi di jalur ujian (S9), bukan halaman katalog
- **Rilis** — deploy staging, UAT klien mulai 30 Juli, bug fixing, go-live 3 Agustus
- **AC**
  - 1.000 CCU: p95 API ≤500ms non-ujian, load soal ≤1s
  - Restore backup ke database bersih **berhasil dijalankan**, bukan sekadar backup ada
  - Rate limit terbukti menahan brute-force login
  - Uptime & alerting aktif sebelum go-live

---

## 5. Ringkasan Integrasi

Task integrasi tersebar di slice pemakainya, tapi dikumpulkan di sini untuk keperluan penjadwalan pihak ketiga:

| Integrasi | Slice | Blocker |
|---|---|---|
| Kontrak OpenAPI + typed client | S0 | — |
| Cabut `lib/mock-data.ts` per domain | S1, S3, S5, S6, S7, S8, S9, S10, S12 | — |
| Email dev/UAT via akun owner | S2 | **Selesai — diputuskan 2026-07-23** |
| Email produksi (domain klien + SPF/DKIM) | S13 | Butuh akses domain klien — **syarat rilis** |
| **Provider WhatsApp** | S2 | **Belum diputuskan — lihat §6 #1** |
| Midtrans (sandbox → prod) | S6 | Butuh akun merchant klien + verifikasi bisnis |
| Object storage MinIO/GCS | S5, S6, S7 | — |
| Redis (ranking + cache subtes) | S9, S10 | — |
| River job queue | S2, S10 | — |
| Cloudflare CDN + pemetaan domain | S4, S13 | **Lihat §6 #2** |

**Dua yang bergantung pihak lain — mulai proses akuisisinya di Minggu 1, jangan menunggu slice-nya tiba:** akun merchant Midtrans (verifikasi bisnis bisa makan berhari-hari) dan akses domain untuk DNS email.

---

## 6. Keputusan Terbuka

**Sudah diputuskan 2026-07-23:**

- **OTP dipakai** (menyimpang dari PRD §7 yang menyatakan tanpa OTP) — pengiriman via email dulu. → S1, S2. *PRD §7 perlu diperbarui agar tidak bertentangan dengan implementasi.*
- **Pengirim email dev/UAT** = akun owner, dengan migrasi ke domain klien sebagai syarat rilis di S13. → S2

Tiga hal yang masih menghambat slice tertentu dan butuh jawaban klien:

1. **Provider & channel WhatsApp (FR-NOT-04).** PRD merujuk "§10 Open Questions #6" yang **sudah tidak ada** di PRD v1.2 — rujukan menggantung. Perlu ditetapkan providernya (WA Business API resmi vs penyedia pihak ketiga), karena harga & waktu verifikasinya jauh berbeda. **Menghambat:** S2 selesai penuh. *Mitigasi: interface notifier dengan implementasi no-op supaya slice lain tetap jalan.*
2. **Pemisahan domain.** PRD §5.19 merujuk "§10 Open Question #5" yang juga **sudah tidak ada**. Perlu konfirmasi: `tarunaeduka.id` (landing) vs `tryout.tarunaeduka.id` (aplikasi), dan siapa yang pegang DNS. **Menghambat:** S4, S13.
3. **Formula skor Kecermatan (§10 #1).** Masih provisional. Sumber faktor `−0,05` pada Kecepatan & Ketahanan belum dijelaskan klien. **Tidak menghambat** — implementasikan sebagai baseline, tandai provisional di UI, tapi konfirmasi sebelum UAT supaya tidak jadi temuan di menit terakhir.

---

## 7. Catatan Sizing

Ukuran bersifat relatif (S < M < L < XL), bukan estimasi hari — pemetaan ke kalender mengikuti RAB v1.3 (±5 minggu, Rp 57jt).

Distribusinya berat sebelah, dan itu memang mencerminkan kenyataan: **S9 (Pengerjaan Tryout) sendirian menanggung porsi risiko terbesar**, konsisten dengan RAB yang menaruh Rp 28,5jt di Modul 3 dan menandainya "modul paling kritikal". S6 (Checkout) urutan kedua karena menyentuh uang dan pihak ketiga.

Dua hal yang perlu dijaga sepanjang eksekusi:

- **Jalur kritis tidak bisa dipendekkan dengan menambah orang di slice lain.** S0 → S1 → S3 → S7 → S8 → S9 → S10 berurutan. Orang tambahan berguna di S2, S4, S11, S12 — bukan di jalur ini.
- **Mockup FE yang sudah 85% jadi memang memangkas kerja FE, tapi tidak memangkas S9.** Layar ujian sudah ada; yang belum ada adalah timer otoritatif server, pengacakan, dan perilaku kehilangan koneksi — dan semuanya berada di sisi backend.

---

## 8. Realitas Jadwal (per 2026-07-23)

Bagian ini ditulis untuk dipakai saat presentasi ke klien.

**Angka yang perlu disepakati di depan:**

| | |
|---|---|
| Hari ini | Kamis, 23 Juli 2026 |
| Target go-live di PRD & RAB | Senin, 3 Agustus 2026 — **11 hari lagi** |
| Sisa pekerjaan | 14 slice; RAB sendiri menskalakannya ±5 minggu |
| Backend | 0% — belum ada `go.mod`, migration, atau service |

**Target 3 Agustus sudah tidak dapat dicapai.** Ini bukan soal kecepatan kerja: RAB v1.3 menganggarkan ±5 minggu untuk lingkup ini, dan yang tersedia 1,5 minggu. Menyampaikan ini di presentasi Senin jauh lebih murah daripada klien menemukannya sendiri pada 3 Agustus.

**Kenapa menambah orang tidak memperbaikinya.** Jalur kritis S0 → S1 → S3 → S7 → S8 → S9 → S10 berurutan secara teknis: bank soal butuh struktur program, builder butuh bank soal, engine ujian butuh builder. S9 sendirian berukuran XL. Orang tambahan mempercepat S2/S4/S11/S12 — tidak satu pun ada di jalur kritis.

**Yang realistis dikerjakan dalam satu minggu penuh:** S0 dan S1 — tulang punggung berjalan, login & OTP asli di atas Postgres. Itu ±2 dari 14 slice, dan tidak satu pun menghasilkan sesuatu yang terlihat berbeda di layar dibanding mockup yang sudah ada.

### 8.1 Untuk presentasi Senin (4 hari — realistis 1 hari kerja + akhir pekan)

**Jangan mulai backend untuk demo ini.** Login JWT asli dan login mock terlihat **persis sama** di depan klien. Tiga hari kerja backend menghasilkan nol perbedaan yang bisa dilihat penonton.

Aset terkuat yang sudah dimiliki adalah mockup 34 halaman yang jalan di browser. Yang bernilai justru menutup titik buntu yang akan terlihat saat alur didemokan:

| Prioritas | Pekerjaan | Kenapa |
|---|---|---|
| 1 | Checkout menyimpan ke mock store (order + enrollment + potong saldo) | Titik buntu paling terlihat — saat ini tombol bayar tidak menghasilkan apa-apa. Frontend saja, hitungan jam |
| 2 | Toggle acak soal & jawaban muncul di tryout builder | Fitur yang ditagih di RAB Modul 3; kalau tidak ada di layar, klien menganggapnya belum dikerjakan |
| 3 | Halaman riwayat saldo | Melengkapi cerita 3 metode pembayaran yang jadi nilai jual Modul 2 |
| 4 | Latih alur demo ujung-ke-ujung | Daftar → beli → kerjakan → lihat hasil, tanpa menabrak layar yang belum tersambung |

Semuanya berada di `lib/mock-data.ts` dan komponen yang sudah ada — tidak menyentuh backend, tidak membuang pekerjaan, karena semua tetap terpakai sebagai kondisi awal saat backend menyusul.

### 8.2 Bahan diskusi jadwal dengan klien

Tiga arah yang bisa ditawarkan; pilihannya milik klien, bukan milik tim:

1. **Geser go-live ke ±akhir Agustus**, lingkup penuh utuh sesuai PRD. Paling jujur, tidak ada yang dikorbankan.
2. **Rilis bertahap pakai batas modul RAB yang sudah ada.** Modul 1+2 (akun, katalog, transaksi) tayang lebih dulu — siswa sudah bisa mendaftar dan membeli — lalu Modul 3+4 menyusul. Cocok kalau 3 Agustus terikat komitmen pemasaran klien.
3. **Kurangi lingkup MVP.** Kandidat paling masuk akal untuk ditunda: Tes Kecermatan (S12, Modul 4, Rp 7,5jt) — produk terpisah yang tidak menghalangi alur tryout, dan formula skornya pun masih menunggu validasi klien.

Opsi 2 dan 3 bisa digabung. Yang tidak tersedia adalah mengerjakan lingkup penuh sebelum 3 Agustus.

---

**Document End**
