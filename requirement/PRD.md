# Product Requirements Document (PRD)
## Platform Tryout Online — Taruna Eduka

**Versi:** 1.2
**Tanggal:** Juli 2026
**Owner:** Taruna Eduka
**Status:** Draft
**Target Go-Live:** 3 Agustus 2026

---

## 1. Executive Summary

Platform Tryout Online adalah aplikasi web yang memungkinkan siswa membeli dan mengerjakan paket tryout secara mandiri (self-paced), tanpa event ujian live atau kompetisi nasional. Platform dirancang khusus untuk bimbel berskala menengah dengan program lintas jenjang: SD, SMP, SMA, CASN, BUMN, dan AKPOL.

**Value Proposition:**
- Siswa bisa beli dan kerjakan tryout kapan saja, tanpa harus hadir fisik
- Admin operasional cukup 1 role — tim 3-5 orang
- Hasil tryout lengkap: skor per subtes, ranking internal, pembahasan soal
- Sistem penilaian fleksibel: weighted scoring, bobot per opsi jawaban, atau IRT (opsional per tryout)
- Tryout dapat dikerjakan berulang — leaderboard berbasis rata-rata seluruh percobaan
- **Tes Kecermatan** sebagai format latihan psikotes tambahan di luar tryout — dijual dan diakses terpisah (lihat §5.17)

**Model Pembayaran (aktif bersamaan, siswa memilih saat checkout):**
- **Payment Gateway:** siswa checkout via Midtrans, akses terbuka otomatis
- **Konfirmasi Manual:** siswa order + upload bukti transfer, admin konfirmasi manual
- **Saldo (Wallet):** siswa membayar dari saldo yang tersimpan di akun, akses terbuka otomatis

---

## 2. Goals & Objectives

### 2.1 Product Goals
1. Siswa dapat membeli dan mengerjakan tryout secara mandiri dari perangkat apapun
2. Admin dapat mengelola seluruh operasional (soal, tryout, produk, siswa, pesanan) dari satu dashboard
3. Hasil tryout valid, transparan, dan mudah dipahami siswa

### 2.2 Non-Goals (MVP)
- Tidak ada live class atau video pembelajaran
- Tidak ada ranking nasional (ranking hanya dalam bimbel)
- Tidak ada fitur multi-tenant atau multi-cabang
- Tidak ada event ujian terjadwal atau kompetisi
- Tidak ada aplikasi mobile native (Android/iOS) — web responsive cukup untuk MVP

---

## 3. Target Users & Personas

### 3.1 Persona 1 — Siswa (Primary User)
- **Jenjang:** SD / SMP / SMA / Persiapan CASN / BUMN / AKPOL
- **Kebutuhan:** Latihan soal tryout sesuai jenjang, tahu skor dan letak kesalahan, bisa belajar mandiri
- **Pain Points:** Tidak tahu soal mana yang lemah, hasil tryout tidak langsung keluar
- **Device:** Smartphone (mayoritas) dan laptop

### 3.2 Persona 2 — Operator Bimbel
- **Profil:** Staf bimbel yang menangani operasional harian
- **Kebutuhan:** Konfirmasi pembayaran cepat, tambah/edit data siswa, Bikin soal dan tryouts, lihat laporan hasil
- **Pain Points:** Harus minta izin pemilik untuk hal-hal operasional sederhana

### 3.3 Persona 3 — Admin / Pemilik Bimbel
- **Profil:** Pemilik atau pengelola utama bimbel
- **Kebutuhan:** Kontrol penuh atas konten (soal, tryout, produk), konfigurasi sistem, laporan keseluruhan
- **Pain Points:** Operasional manual via WhatsApp dan spreadsheet, sulit track siapa yang sudah bayar

---

## 4. User Roles & Permissions

| Role | Akses |
|---|---|
| **Student** | Browse katalog, beli produk, kerjakan tryout, lihat hasil & ranking, lihat pembahasan soal |
| **Operator** | Kelola konten (soal/tryout/produk) + operasional (pesanan, siswa, hasil) — tidak bisa kelola program, akun user, dan konfigurasi sistem |
| **Admin** | Full access: semua akses Operator + manajemen program, akun user (buat/hapus operator & siswa), konfigurasi sistem |

---

## 5. Functional Requirements

### 5.1 Auth & Registrasi
- **FR-AUTH-01:** Registrasi siswa via email **atau nomor HP** + password
- **FR-AUTH-02:** Login menggunakan **email atau nomor HP** + password untuk Student, Operator, dan Admin
- **FR-AUTH-03:** Forgot password via email
- **FR-AUTH-04:** Session JWT, auto-logout setelah idle
- **FR-AUTH-05:** Admin dapat membuat, mengedit, dan menghapus akun siswa dan operator dari dashboard

### 5.2 Katalog & Produk
- **FR-CAT-01:** Halaman katalog publik menampilkan semua produk aktif
- **FR-CAT-02:** Filter produk berdasarkan program (SD/SMP/SMA/CASN/BUMN/AKPOL)
- **FR-CAT-03:** Tiga tipe produk:
  - **Individual** — 1 tryout, aktif hingga tanggal yang ditetapkan admin
  - **Bundle** — N tryout dalam satu paket dengan harga diskon, tanggal aktif sama
  - **Kecermatan** — 1 tes kecermatan, memberikan sejumlah attempt (bukan akses tak terbatas) — lihat §5.17
- **FR-CAT-04:** Halaman detail produk: judul, deskripsi, daftar materi/subtes, harga normal, harga diskon, tanggal aktif
- **FR-CAT-05:** Tombol "Beli" membawa siswa ke alur pembelian

### 5.3 Pembelian & Pembayaran

Opsi A, Opsi C, dan Saldo **aktif bersamaan** — siswa memilih metode pembayaran saat checkout (bukan dipilih sekali saat setup, seperti di v1.0).

#### Opsi A — Payment Gateway
- **FR-PAY-A-01:** Siswa checkout, redirect ke halaman pembayaran Midtrans
- **FR-PAY-A-02:** Webhook konfirmasi dari gateway → status pesanan otomatis `confirmed` → enrollment terbuka
- **FR-PAY-A-03:** Notifikasi email + WhatsApp ke siswa setelah pembayaran berhasil

#### Opsi C — Konfirmasi Manual
- **FR-PAY-C-01:** Siswa membuat pesanan → status `pending`
- **FR-PAY-C-02:** Siswa upload bukti transfer di halaman pesanan
- **FR-PAY-C-03:** Admin melihat daftar pesanan pending di dashboard, konfirmasi manual → status `confirmed` → enrollment terbuka
- **FR-PAY-C-04:** Admin dapat membatalkan pesanan (status `cancelled`)

#### Saldo (Wallet)
- **FR-PAY-W-01:** Siswa memiliki saldo yang tersimpan di akun; top-up dilakukan manual oleh admin/operator untuk MVP (tidak ada top-up self-service)
- **FR-PAY-W-02:** Saat checkout, siswa dapat membayar menggunakan saldo jika mencukupi — status pesanan otomatis `confirmed`, saldo terpotong sejumlah harga produk
- **FR-PAY-W-03:** Riwayat penggunaan saldo tercatat dan dapat dilihat siswa

#### Umum
- **FR-PAY-01:** Riwayat pesanan dapat dilihat oleh siswa ("Paket Belajar Saya")
- **FR-PAY-02:** Setelah enrollment aktif, siswa dapat mengakses tryout hingga tanggal `active_until`

### 5.4 Pengerjaan Tryout
- **FR-TRY-01:** Daftar tryout yang dimiliki siswa tampil di "Paket Belajar Saya"
- **FR-TRY-02:** Siswa membuka tryout dan melihat daftar subtes beserta durasi masing-masing
- **FR-TRY-03:** Siswa memulai subtes pertama — timer mulai berjalan
- **FR-TRY-04:** Dalam subtes, siswa menjawab soal dan dapat navigasi antar soal bebas
- **FR-TRY-05:** **Blocking time:** setelah subtes di-submit (atau timer habis), tidak bisa kembali ke subtes sebelumnya
- **FR-TRY-06:** Timer habis → subtes auto-submit otomatis
- **FR-TRY-07:** Setelah semua subtes selesai → hasil tryout langsung tampil
- **FR-TRY-08:** **Tryout dapat dikerjakan lebih dari 1 kali** (multiple attempts). Setiap percobaan tersimpan sebagai hasil terpisah (`attempt_number`); siswa dapat melihat riwayat seluruh percobaan pada halaman tryout dan mengulang kapan saja selama enrollment masih aktif
- **FR-TRY-09:** Jika siswa keluar di tengah subtes (browser ditutup/koneksi putus), session tetap aktif dan dapat dilanjutkan selama timer belum habis
- **FR-TRY-10:** Saat siswa memulai subtes, browser otomatis masuk fullscreen mode
- **FR-TRY-11:** Jika siswa keluar dari fullscreen, sistem mencatat pelanggaran (violation log) dengan timestamp dan menampilkan warning overlay yang meminta siswa kembali ke fullscreen — timer tetap berjalan
- **FR-TRY-12:** Tryout memiliki tanggal mulai (`active_from`) dan tanggal berakhir (`active_until`) — siswa hanya dapat mengerjakan dalam rentang tersebut
- **FR-TRY-13:** Selama sesi fullscreen, sistem mendeteksi kombinasi tombol screenshot umum (PrintScreen, Cmd+Shift+3/4/5, Win+Shift+S), memblokir perilaku default browser, menampilkan popup peringatan, dan mencatat pelanggaran (`violation type: screenshot_attempt`). **Catatan teknis:** ini adalah *best-effort deterrent* — screenshot di level OS (mis. utility screenshot bawaan sistem operasi) tidak dapat sepenuhnya dicegah dari dalam browser
- **FR-TRY-14:** Jika tryout mengaktifkan `randomize_questions` (lihat FR-ADM-TRY-06), urutan tampil soal dalam tiap subtes diacak ulang setiap kali siswa memulai attempt baru — urutan diacak sekali di awal session dan tetap konsisten selama session tersebut berjalan (tidak berubah saat siswa navigasi bolak-balik antar soal)
- **FR-TRY-15:** Jika tryout mengaktifkan `randomize_options` (lihat FR-ADM-TRY-07), urutan tampil opsi jawaban untuk soal tipe `pg` dan `pg_multi` diacak per session dengan cara yang sama seperti FR-TRY-14. Bobot poin tiap opsi (lihat §5.6 Bobot per Opsi Jawaban) tetap melekat pada konten opsi, bukan posisi tampil — acak posisi tidak memengaruhi skor

### 5.5 Tipe Soal
Empat tipe soal didukung:

| Tipe | Keterangan | Penilaian |
|---|---|---|
| `pg` | Pilihan ganda, 1 jawaban benar | Benar / Salah |
| `pg_multi` | Pilihan ganda, >1 jawaban benar | All-or-nothing (harus pilih semua jawaban benar) |
| `isian` | Isian singkat (teks) | Pencocokan teks exact / case-insensitive |
| `benar_salah` | Benar atau Salah | Benar / Salah |

Soal dapat menyertakan gambar (image upload oleh admin) dan ekspresi matematika dalam format LaTeX/MathJax.

### 5.6 Sistem Penilaian

#### Weighted Scoring (Default)
- Admin mengatur bobot poin per tingkat kesulitan: Mudah / Sedang / Sulit
- Skor = Σ(jawaban benar × bobot kesulitan soal)
- Konfigurasi bobot bersifat global (berlaku untuk semua tryout dalam mode ini)

#### IRT Mode (Opsional, per Tryout)
- Admin mengaktifkan IRT saat membuat tryout
- Perhitungan tetap menggunakan weighted scoring berbasis kesulitan
- Sistem menambahkan **prevalidation warning**: jika ada soal dengan respons < 30 siswa, flag `irt_warning = true` disimpan pada hasil dan ditampilkan di dashboard admin
- Tujuan: transparansi bahwa data belum cukup untuk kalibrasi IRT yang valid secara statistik

#### Bobot per Opsi Jawaban (Opsional, per Soal)
- Admin dapat mengaktifkan mode "bobot per opsi" pada soal tipe pilihan ganda (`pg`) — semua opsi dianggap valid/benar, masing-masing diberi bobot poin berbeda (mis. a:4, b:3, c:2, d:1)
- Skor siswa untuk soal tersebut = bobot poin dari opsi yang dipilih (bukan benar/salah)
- Opsi yang belum diberi bobot secara eksplisit oleh admin otomatis bernilai default **1**
- Cocok untuk soal tipe TKP/psikotes yang secara desain tidak memiliki jawaban salah mutlak

### 5.7 Hasil & Ranking
- **FR-RES-01:** Halaman hasil menampilkan: skor total, skor per subtes, ranking di antara semua peserta tryout yang sama
- **FR-RES-02:** Riwayat hasil dapat dilihat kembali oleh siswa kapan saja, termasuk seluruh percobaan bila tryout dikerjakan lebih dari 1 kali (lihat FR-TRY-08)
- **FR-RES-03:** Pembahasan soal (teks/gambar per soal) tampil setelah tryout selesai, **kecuali admin menahan tampilan pembahasan hingga tanggal tertentu** (`explanation_release_at`) — selama ditahan, siswa hanya melihat skor
- **FR-RES-04:** Ranking bersifat internal bimbel (bukan nasional)
- **FR-RES-05:** Karena tryout dapat dikerjakan berulang, leaderboard/ranking per tryout dihitung dari **rata-rata skor seluruh percobaan siswa** pada tryout tersebut, bukan skor satu percobaan saja

### 5.8 Notifikasi Email & WhatsApp
- **FR-NOT-01:** Email konfirmasi pesanan ke siswa setelah order dibuat (Opsi C/Saldo: saat pending/confirmed; Opsi A: saat confirmed)
- **FR-NOT-02:** Email notifikasi akses terbuka ke siswa setelah enrollment aktif
- **FR-NOT-03:** Email forgot password (reset link)
- **FR-NOT-04:** Notifikasi WhatsApp untuk event yang sama seperti FR-NOT-01/02 (konfirmasi pesanan, akses terbuka) — channel non-email sesuai keputusan klien (lihat §10 Open Questions #6)

### 5.9 Admin — Manajemen Program
- **FR-ADM-PRG-01:** CRUD program (SD, SMP, SMA, CASN, BUMN, AKPOL dan lainnya) — **hanya dapat diakses oleh role Admin**, tidak oleh Operator (perubahan dari v1.0, yang mengizinkan Operator)
- **FR-ADM-PRG-02:** Setiap program memiliki daftar mata pelajaran; setiap mata pelajaran memiliki daftar bab materi (chapter). Struktur ini menjadi sumber dropdown "mata pelajaran" dan "bab materi" saat membuat soal (lihat FR-ADM-SOA-01)

### 5.10 Admin & Operator — Bank Soal
- **FR-ADM-SOA-01:** CRUD soal: input tipe, konten, mata pelajaran, **bab materi (dropdown, opsional)**, opsi jawaban, jawaban benar (atau bobot per opsi — lihat §5.6), tingkat kesulitan, penjelasan, gambar
- **FR-ADM-SOA-02:** Soal mendukung ekspresi matematika via LaTeX/MathJax (render live saat input)
- **FR-ADM-SOA-03:** Filter soal berdasarkan program, tipe, dan kesulitan
- **FR-ADM-SOA-04:** Import soal massal via file **Excel/CSV atau Microsoft Word (.docx)** dengan template yang disediakan sistem
- **FR-ADM-SOA-05:** Preview soal sebelum simpan (termasuk render LaTeX)
- **FR-ADM-SOA-06:** ID unik tiap soal ditampilkan di daftar bank soal untuk memudahkan referensi/QA

### 5.11 Admin & Operator — Tryout Builder
- **FR-ADM-TRY-01:** Buat tryout: judul, program, mode penilaian (weighted/IRT), **tanggal mulai (`active_from`) dan tanggal berakhir (`active_until`)**, cover image
- **FR-ADM-TRY-02:** Tambah subtes ke tryout: nama subtes, durasi (menit), urutan
- **FR-ADM-TRY-03:** Assign soal ke subtes: pilih dari bank soal, atur urutan tampil
- **FR-ADM-TRY-04:** Edit dan hapus tryout (hanya jika belum ada siswa yang mengerjakan)
- **FR-ADM-TRY-05:** Admin dapat menahan tampilan pembahasan (`explanation_release_at`) — jika diisi, pembahasan baru tampil ke siswa setelah tanggal tersebut; sebelum itu siswa hanya melihat skor (lihat FR-RES-03)
- **FR-ADM-TRY-06:** Toggle "Acak Urutan Soal" (`randomize_questions`) per tryout — lihat FR-TRY-14
- **FR-ADM-TRY-07:** Toggle "Acak Urutan Jawaban" (`randomize_options`) per tryout, berlaku untuk soal tipe `pg` dan `pg_multi` — lihat FR-TRY-15

### 5.12 Admin & Operator — Manajemen Produk
- **FR-ADM-PRD-01:** Buat produk individual: pilih 1 tryout, set harga, harga coret, tanggal aktif, cover image
- **FR-ADM-PRD-02:** Buat produk bundle: pilih N tryout, set harga bundle, harga coret, tanggal aktif
- **FR-ADM-PRD-03:** Aktifkan / nonaktifkan produk dari katalog publik

### 5.13 Admin & Operator — Manajemen Pesanan
- **FR-ADM-ORD-01:** Lihat semua pesanan dengan filter status (pending / confirmed / cancelled)
- **FR-ADM-ORD-02:** (Opsi C) Konfirmasi atau batalkan pesanan pending
- **FR-ADM-ORD-03:** Lihat bukti transfer yang diupload siswa

### 5.14 Admin & Operator — Manajemen Siswa
- **FR-ADM-STU-01:** CRUD akun siswa
- **FR-ADM-STU-02:** Lihat daftar enrollment aktif per siswa
- **FR-ADM-STU-03:** Grant akses tryout manual (tanpa pesanan) untuk kasus khusus

### 5.15 Admin & Operator — Dashboard Hasil
- **FR-ADM-RES-01:** Lihat hasil semua siswa per tryout: skor, ranking, timestamp pengerjaan
- **FR-ADM-RES-02:** Filter hasil berdasarkan tryout dan program
- **FR-ADM-RES-03:** Indikator IRT warning jika berlaku
- **FR-ADM-RES-04:** Export hasil ke Excel/CSV

### 5.16 Admin & Operator — Live Monitoring & Violation Log
- **FR-MON-01:** Halaman monitoring menampilkan daftar siswa yang sedang aktif dalam sesi tryout (`status = in_progress`): nama siswa, tryout yang dikerjakan, subtes aktif, waktu mulai, jumlah pelanggaran
- **FR-MON-02:** Halaman auto-refresh setiap 30 detik (polling — tidak perlu WebSocket)
- **FR-MON-03:** Admin & Operator dapat membuka detail sesi siswa: timeline pelanggaran (tipe + timestamp), total pelanggaran per sesi
- **FR-MON-04:** Riwayat violation log juga tersedia untuk sesi yang sudah selesai di halaman hasil

### 5.17 Kecermatan Test (Format Baru — Terpisah dari Tryout)

Format latihan psikotes baru, dijual dan dikerjakan terpisah dari tryout (produk sendiri di katalog, bukan subtes di dalam tryout). Referensi mekanik: fitur "Kecermatan Simbol Hilang" pada platform CBT existing milik klien.

**Mekanik pengerjaan:**
- **FR-KEC-01:** Satu tes kecermatan terdiri dari N **kolom** (bagian); tiap kolom berisi M **soal** dan punya timer bersama (`kolom_duration_seconds`) untuk seluruh soal di kolom tersebut
- **FR-KEC-02:** Di awal tiap kolom, sistem membangkitkan satu set item referensi berlabel (mis. A–E) secara acak — set ini **tetap sama** untuk seluruh soal dalam kolom tersebut
- **FR-KEC-03:** Tiap soal menampilkan ulang subset item referensi (urutan diacak) dengan **tepat 1 item dihilangkan**; siswa memilih label item mana yang hilang
- **FR-KEC-04:** Soal yang tidak dijawab saat timer kolom habis dicatat sebagai `kosong` (bukan salah)
- **FR-KEC-05:** Setelah kolom selesai (semua soal terjawab atau timer habis), sistem menampilkan jeda transisi singkat dengan tombol "Lanjut" dan countdown otomatis ke kolom berikutnya
- **FR-KEC-06:** Tidak ada fullscreen enforcement, violation log, atau ranking/leaderboard untuk format ini (berbeda dari tryout)

**Attempt & billing:**
- **FR-KEC-07:** Pembelian produk kecermatan memberikan sejumlah **attempt** (`included_attempts`), bukan akses tak terbatas hingga tanggal tertentu seperti tryout
- **FR-KEC-08:** Tiap attempt mengurangi kuota attempt siswa untuk tes tersebut; jika kuota habis, siswa diarahkan kembali ke checkout untuk membeli attempt tambahan (menambah kuota, bukan transaksi baru yang terpisah)
- **FR-KEC-09:** Checkout kecermatan menggunakan 3 metode pembayaran yang sama seperti tryout (Gateway/Manual/Saldo — lihat §5.3)
- **FR-KEC-10:** Siswa dapat melihat riwayat seluruh attempt yang sudah dikerjakan dan sisa kuota attempt yang tersedia

**Hasil:**
- **FR-KEC-11:** Hasil per attempt menampilkan jumlah Benar/Salah/Kosong per kolom dan total
- **FR-KEC-12:** Hasil per attempt juga menampilkan 3 skor komposit: **Kecepatan**, **Ketahanan**, **Ketelitian** — skala 0–100. **Catatan:** formula belum divalidasi final dengan klien — lihat §10 Open Questions #7 untuk formula yang dipakai
- **FR-KEC-13:** Pembahasan per attempt: siswa dapat membuka rincian tiap soal (set referensi, subset yang ditampilkan, item yang hilang, jawaban siswa, benar/salah)
- **FR-KEC-14:** Hasil per attempt juga menampilkan **Nilai Akhir** — skor gabungan tertimbang dari Kecepatan (30%), Ketelitian (45%), Ketahanan (25%). Lihat §10 Open Questions #7 untuk formula

### 5.18 Admin & Operator — Kecermatan Builder & Manajemen
- **FR-ADM-KEC-01:** Buat tes kecermatan: program, judul, tipe item (`char`/`number`/`symbol`/`emoji`/`mix`), panjang item (mis. 4 karakter), jumlah opsi per soal, jumlah soal per kolom, jumlah kolom, durasi timer per kolom, cover image
- **FR-ADM-KEC-02:** Buat produk kecermatan dari tes yang sudah dibuat: set harga, harga coret, dan jumlah `included_attempts` per pembelian
- **FR-ADM-KEC-03:** Edit dan hapus tes kecermatan (hanya jika belum ada siswa yang mengerjakan)
- **FR-ADM-KEC-04:** Lihat hasil semua siswa per tes kecermatan: Benar/Salah/Kosong, 3 skor komposit, Nilai Akhir, timestamp pengerjaan, sisa kuota attempt
- **FR-ADM-KEC-05:** Filter hasil berdasarkan tes kecermatan dan program

### 5.19 Landing Page (Marketing)

Halaman publik di domain root (`tarunaeduka.id`) — berbeda dari halaman katalog (`/catalog`, lihat §5.2), berfungsi sebagai halaman perkenalan/promosi produk sebelum siswa masuk ke katalog atau mendaftar. Diserve terpisah dari aplikasi tryout (`tryout.tarunaeduka.id`, lihat §10 Open Question #5).

- **FR-LAND-01:** Hero section — headline, deskripsi singkat, CTA "Daftar Sekarang" (→ registrasi) dan "Lihat Katalog Tryout" (→ katalog), serta ringkasan angka (jumlah program, siswa terdaftar, kecepatan hasil)
- **FR-LAND-02:** Daftar program diklat unggulan (TNI, POLRI, Kedinasan) dengan tautan ke katalog terfilter per program
- **FR-LAND-03:** Section keunggulan platform (soal HOTS, sistem CAT, hasil instan, soal dapat diunduh, analisa detail per butir soal, harga kompetitif)
- **FR-LAND-04:** Produk unggulan — menampilkan beberapa produk dari katalog (harga, program) dengan tautan ke detail produk
- **FR-LAND-05:** Section "Cara Kerja" — 3 langkah ringkas (pilih & beli paket, kerjakan sesuai timer, lihat hasil & pembahasan)
- **FR-LAND-06:** CTA penutup dan footer, termasuk tautan ke portal internal (Operator/Admin)

---

## 6. Non-Functional Requirements

### 6.1 Performance
- API response p95 ≤ 500ms untuk operasi non-ujian
- Halaman tryout (load soal) ≤ 1s
- Auto-submit saat timer habis: toleransi ±5 detik

### 6.2 Availability
- Uptime target 99.5% (downtime ≤ 3.6 jam/bulan)
- Database backup harian, retention 30 hari

### 6.3 Security
- HTTPS untuk semua traffic
- Password hashing (bcrypt)
- JWT dengan expiry + refresh
- Rate limiting pada endpoint auth dan submit jawaban
- Upload file: validasi tipe dan ukuran (max 5MB per gambar)

### 6.4 Usability
- Responsive web (mobile-first) — tidak ada app native untuk MVP
- Tampilan timer tryout jelas dan selalu terlihat
- Warning sebelum submit subtes (tidak bisa kembali)

### 6.5 Scalability
- Target: 10.000 pengguna terdaftar, puncak 1.000 concurrent users (CCU)
- Desain stateless untuk memudahkan scaling horizontal jika diperlukan

---

## 7. Technical Architecture

| Layer | Pilihan |
|---|---|
| Backend | Go — Echo v4, pgx/v5, raw SQL, golang-migrate |
| Frontend | Next.js 15, React 19, Tailwind v4 — self-hosted, disajikan lewat Cloudflare (CDN + free tier, lihat §7.2) |
| Auth | JWT (tanpa OTP — tidak diperlukan untuk skala ini) |
| Storage | MinIO (lokal) / GCS (produksi) — gambar soal + bukti transfer |
| Payment | Midtrans (gateway) + konfirmasi manual — keduanya aktif bersamaan; saldo/wallet internal sebagai metode tambahan |
| Async & Cache | Redis (Sorted Set ranking + cache konten subtes) + River — Postgres-backed job queue untuk event kritikal (lihat §7.1) |
| Database | PostgreSQL |

**Package layout backend:**
`model → repository → service → handler/server`, dengan `infra` dan `adapter` untuk third-party.

### 7.1 Async & Event Architecture

- **Event dispatch:** in-process event bus (pola Mediator) — `Publish(event)` memanggil handler yang terdaftar untuk tipe event tersebut, decoupled dari publisher. Satu API yang sama dipakai baik untuk event kritikal maupun non-kritikal; yang berbeda adalah isi handler-nya.
- **Event kritikal** (harus survive crash/restart) — notifikasi email/WhatsApp (FR-NOT-01/02/04), export hasil (FR-ADM-RES-04): handler hanya melakukan insert job ke River **dalam transaction yang sama** dengan perubahan state bisnis terkait (mis. order menjadi `confirmed`). Job baru tercatat kalau dan hanya kalau transaction commit — tidak ada dual-write ke sistem terpisah, tidak ada risiko job hilang kalau proses crash di antara update Postgres dan pengiriman notifikasi.
- **Event non-kritikal** (murah dihitung ulang, boleh hilang) — update ranking/leaderboard, refresh live-monitoring (FR-MON-01): handler jalan langsung in-memory, tanpa persist ke mana pun.
- **River** (`github.com/riverqueue/river`, Postgres-backed, dibangun di atas `pgx`): tabel job disimpan di schema Postgres terpisah (`river`), bukan schema bisnis utama — memisahkan migration tracker dan permission dari tabel domain. Retention cleanup otomatis menghapus job `completed`/`discarded` setelah retention window, sehingga tabel tidak membengkak meski volume user bertambah.
- **Redis** dipakai hanya untuk data derived/recomputable: Sorted Set untuk ranking (FR-RES-05 — rata-rata skor seluruh percobaan siswa) dan cache konten subtes pre-shuffle per `subtest_id`. **Bukan** dipakai sebagai job queue — push ke Redis tidak bisa ikut transaction Postgres yang sama (reintroduce dual-write problem), dan restart VM/pod menghapus seluruh isi Redis; risiko kehilangan notifikasi pembayaran ini tidak dapat diterima.

### 7.2 CDN & Hosting

- **Hosting:** Next.js di-deploy self-hosted (bukan platform managed seperti Vercel). Vercel Hobby tier (gratis) melarang penggunaan komersial di ToS-nya; tier berbayar (Pro) mulai ~$20+/seat/bulan plus biaya pemakaian — belum dianggarkan di RAB. Cloudflare dipilih sebagai lapisan CDN + hosting karena free tier-nya tidak membatasi penggunaan komersial, dan tetap dalam model billing GCP-at-cost yang sudah ada tanpa menambah vendor berbayar baru.
- **Scope caching:** CDN/edge cache hanya berlaku untuk aset statis (bundle JS/CSS, file gambar) yang URL-nya stabil per file — tidak terpengaruh oleh pengacakan urutan soal/jawaban (FR-TRY-14/15). Response API konten ujian **tidak pernah** di-cache CDN (`Cache-Control: no-store`), karena kontennya diacak per sesi siswa — kalau ikut ter-cache, semua siswa akan menerima urutan acak yang sama (cache hit), yang membatalkan tujuan randomisasi. Ini layer yang berbeda dari Redis subtest-content cache di §7.1 (cache app-tier atas data pre-shuffle, tidak terlihat oleh HTTP/CDN).

---

## 8. Data Model

`tryouts` dan `kecermatan_tests` disatukan menjadi satu tabel `tests` (dengan tabel ekstensi per tipe), begitu juga enrollment dan session — lihat catatan di bawah tabel untuk alasannya. `answers`/`results` tetap terpisah per tipe karena bentuk datanya berbeda secara substansial (lihat catatan).

```
programs            id, name, created_at
program_subjects    id, program_id, name
subject_chapters    id, subject_id, name

questions           id, program_id, subject_id, chapter_id (nullable),
                    type(pg|pg_multi|isian|benar_salah),
                    content, options jsonb (each option: id, label, points nullable),
                    correct_answer jsonb, option_weighted bool default false,
                    difficulty(easy|medium|hard), weight, explanation,
                    image_url, created_at

tests               id, program_id, type(tryout|kecermatan), title, description,
                    cover_image_url, created_at

tryout_details      test_id (PK, FK -> tests.id),
                    scoring_mode(weighted|irt), active_from (nullable), active_until,
                    explanation_release_at (nullable),
                    randomize_questions bool default false, randomize_options bool default false
subtests            id, test_id, name, duration_seconds, order
subtest_questions   id, subtest_id, question_id, order

kecermatan_details  test_id (PK, FK -> tests.id),
                    item_type(char|number|symbol|emoji|mix), item_length,
                    options_per_soal, soal_per_kolom, kolom_count, kolom_duration_seconds

products            id, type(individual|bundle|kecermatan), title, description,
                    price, original_price, cover_image_url,
                    active_until, is_active, created_at
product_tests       id, product_id, test_id, included_attempts (nullable, dipakai hanya utk type=kecermatan)

violations          id, session_id, type(exit_fullscreen|screenshot_attempt), created_at

orders              id, user_id, product_id,
                    status(pending|confirmed|cancelled),
                    payment_mode(gateway|manual|wallet),
                    payment_proof_url, confirmed_at, created_at

enrollments         id, user_id, test_id, order_id,
                    active_until (nullable), attempts_purchased (nullable), attempts_used default 0,
                    created_at

test_sessions       id, user_id, test_id, attempt_number,
                    status(in_progress|completed),
                    progress jsonb,
                    started_at, completed_at
                    UNIQUE(user_id, test_id, attempt_number)

tryout_answers      id, session_id, subtest_id, question_id,
                    selected_answer jsonb, is_correct, score
tryout_results      id, session_id, attempt_number, total_score, scoring_mode,
                    subtest_scores jsonb, rank, irt_warning bool

kecermatan_answers  id, session_id, kolom_number, soal_number,
                    displayed_order jsonb, missing_label, selected_label (nullable = kosong),
                    is_correct
kecermatan_results  id, session_id, attempt_number,
                    benar_per_kolom jsonb, salah_per_kolom jsonb, kosong_per_kolom jsonb,
                    benar_total, salah_total, kosong_total,
                    kecepatan_score, ketahanan_score, ketelitian_score, nilai_akhir_score

wallet_transactions id, user_id, type(topup|deduction), amount, order_id (nullable),
                    created_at

users               id, name, email, phone, password_hash,
                    role(student|operator|admin), balance numeric default 0,
                    created_at
```

**Catatan desain (2026-07-04):**
- `tests` menyatukan katalog tryout dan kecermatan (base fields yang genuinely sama: id, program, title, description, cover, created_at). Field yang khusus per tipe dipisah ke `tryout_details`/`kecermatan_details` (extension table 1:1) — bukan kolom nullable campur di satu tabel, karena masing-masing tipe punya field yang sama sekali gak relevan buat tipe lainnya.
- `enrollments` dan `test_sessions` juga disatukan — akses sekarang generik (`active_until` dan `attempts_purchased`/`attempts_used` sama-sama nullable, dipakai sesuai kebutuhan produk, bukan lagi "tryout selalu tanggal, kecermatan selalu attempt"). `test_sessions.progress` isinya jsonb yang bentuknya tergantung `test.type` (progress per subtes utk tryout, `kolom_snapshots` utk kecermatan).
- `tryout_answers`/`tryout_results` **sengaja tidak** disatukan dengan `kecermatan_answers`/`kecermatan_results` — bentuk datanya beda signifikan (jawaban tryout mereferensikan bank soal riil via `question_id`; jawaban kecermatan mereferensikan item hasil generate on-the-fly, gak ada bank soal di baliknya). Menyatukan keduanya akan mengubur `question_id` di dalam jsonb, menyulitkan fitur Post-MVP §9 "identifikasi soal yang paling sering salah" yang butuh agregasi bersih per soal.
- Konsekuensi: `product_tryouts` + `product_kecermatan_tests` (2 junction table terpisah) jadi 1 `product_tests`, karena sekarang cuma ada 1 tabel target (`tests`) buat di-link dari `products`.

---

## 9. Roadmap

### MVP (Target: 3 Agustus 2026)
Seluruh fitur pada dokumen ini adalah scope MVP.

### Post-MVP (Fase 2 — opsional)
- Analitik lanjutan: identifikasi soal yang paling sering salah, prediksi kelulusan
- Migrasi ke multi-tenant jika klien ingin menjual platform ke bimbel lain
- Top-up saldo self-service oleh siswa (MVP: top-up manual oleh admin/operator saja — lihat FR-PAY-W-01)

---

## 10. Open Questions

1. **Formula skor Kecepatan/Ketahanan/Ketelitian/Nilai Akhir untuk Tes Kecermatan (§5.17, FR-KEC-12, FR-KEC-14) — belum divalidasi final dengan klien.** Klien mengirimkan dokumen contoh perhitungan (2026-07-05) dengan formula berikut, menggantikan asumsi skala-maksimal-80 sebelumnya (contoh perhitungan klien sendiri tidak nge-cap di 80 — Ketahanan pada contoh keluar 92,79):
   - Kecepatan = ((total soal dikerjakan / total soal keseluruhan) − 0,05) × 100
   - Ketahanan = ((rata-rata Benar 5 kolom terakhir / rata-rata Benar 5 kolom pertama) − 0,05) × 100
   - Ketelitian = (total Benar / total soal dikerjakan) × 100
   - Nilai Akhir = (0,3 × Kecepatan) + (0,45 × Ketelitian) + (0,25 × Ketahanan)

   Catatan: "total soal dikerjakan" = Benar + Salah (tidak termasuk Kosong). Sumber `−0,05` pada Kecepatan dan Ketahanan belum dijelaskan klien — kemungkinan faktor koreksi/penalti, perlu dikonfirmasi. Formula ini dipakai sebagai baseline implementasi tapi tetap ditandai provisional sampai klien mengonfirmasi secara eksplisit bahwa ini formula final.

---

**Document End**
