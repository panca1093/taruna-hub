# RAB — Platform Tryout Online

| | |
|---|---|
| **Versi** | v1.4 |
| **Tanggal** | 24 November 2026 |
| **Klien** | Taruna Eduka |
| **Lingkup** | Platform tryout self-paced (web) — SD/SMP/SMA/CASN/BUMN/AKPOL, ditambah format Tes Kecermatan |

> Notes feedback dengan klien: 
> - **3 metode aktif bersamaan** — Payment Gateway (Midtrans), Konfirmasi Manual, dan Saldo/Wallet — siswa memilih saat checkout (bukan pilihan eksklusif). 
> - RAB ini mengkuotasikan scope penuh ketiga metode tersebut. 
> - Format tes baru (Tes Kecermatan) dan randomisasi soal/jawaban pada tryout (Modul 3). 

---

## 1. Ringkasan Biaya

| | 3 Metode Pembayaran + Tes Kecermatan |
|---|---|
| Subtotal Development (web) | **Rp 52.500.000** (sekali bayar, setelah penyesuaian harga) |
| Infrastruktur (Tahun 1) | **At-cost** — sesuai tagihan penyedia (Hostinger VPS + Cloudflare) |
| Maintenance (gratis 1 tahun) | Rp 0 |
| **TOTAL YEAR 1** | **Rp 52,5 jt + infrastruktur at-cost** |

---

## 2. Rincian per Modul

| No | Modul | Cakupan / Deliverable                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Estimasi | Catatan |
|---|---|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---|---|
| 1 | **Discovery, Auth & Fondasi** | <ul><li>Requirement gathering & design system</li><li>Arsitektur & schema DB</li><li>Auth JWT 3 role (Student/Operator/Admin) + notifikasi email & WhatsApp</li><li>Shell dashboard admin + portal siswa, light/dark mode</li><li>Landing page marketing (halaman promosi di domain utama, terpisah dari aplikasi tryout — lihat PRD §5.19)</li><li>CRUD Program SD/SMP/SMA/CASN/BUMN/AKPOL</li></ul>                                                                                                                                                                                                          | Rp&nbsp;7.500.000 | Infrastruktur dasar aplikasi, termasuk landing page |
| 2 | **Katalog & Transaksi (3 Metode Pembayaran: Midtrans/Manual/Saldo)** | <ul><li>Katalog & detail produk</li><li>Checkout dengan 3 metode aktif bersamaan:<ul><li>Payment Gateway Midtrans/Xendit dengan konfirmasi otomatis</li><li>Konfirmasi Manual dengan upload bukti transfer</li><li>Saldo/Wallet dengan potong saldo otomatis</li></ul></li><li>Riwayat pesanan siswa</li><li>Admin CRUD produk & manajemen pesanan</li></ul>                                          | Rp&nbsp;13.500.000 | Integrasi payment gateway pihak ketiga, alur konfirmasi manual, dan saldo/wallet |
| 3 | **Tryout Engine** | <ul><li>Bank soal 4 tipe + LaTeX/MathJax + import massal</li><li>Tryout builder, termasuk toggle acak urutan soal & jawaban PG</li><li>Pengerjaan tryout: timer, navigasi, blocking, auto-submit, resume</li><li>Mode layar penuh wajib saat ujian + pencatatan pelanggaran</li><li>Scoring weighted + IRT</li><li>Hasil, ranking & pembahasan</li><li>Live monitoring</li><li>Dashboard hasil admin</li></ul> | Rp&nbsp;25.000.000 | Modul paling kritikal |
| 4 | **Tes Kecermatan (Format Baru)** | <ul><li>Builder tes kecermatan: tipe item, panjang item, opsi/soal, soal/kolom, jumlah kolom, timer</li><li>Engine pengerjaan: generate set referensi per kolom, soal acak-hilang-1, jeda antar-kolom, auto-kosong saat timer habis</li><li>Billing metered per-attempt: beli attempt, potong kuota, beli tambahan</li><li>Hasil B/S/K + 3 skor komposit (Kecepatan/Ketahanan/Ketelitian) + pembahasan</li><li>Dashboard hasil admin</li></ul> | Rp&nbsp;12.500.000 | Produk & billing terpisah dari tryout; formula skor komposit masih perlu validasi klien |
| | **Penyesuaian harga** | | −&nbsp;Rp&nbsp;6.000.000 | Harga yang disepakati bersama klien |
| | **Subtotal Development** |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | **Rp&nbsp;52.500.000** | |
| 5 | **Infrastruktur (Tahun 1)** | <ul><li>Server VPS Hostinger (KVM 2, 8 GB) untuk aplikasi, database dan penyimpanan file</li><li>Cloudflare (paket gratis) untuk keamanan dan kecepatan akses</li><li>Backup rutin ke penyimpanan di luar server</li></ul> | **At-cost** | Ditagihkan sesuai tagihan penyedia, tanpa markup |
| 6 | **Maintenance** | <ul><li>Bug fix fitur dalam scope</li></ul>                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Rp&nbsp;0 | Gratis 1 tahun; setelahnya retainer terpisah |
| | **TOTAL YEAR 1** |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | **Rp&nbsp;52.500.000** | + infrastruktur at-cost |

> Catatan strategi teknis (detail di PRD §7.1):
> - Notifikasi (email/WhatsApp) dan data transaksi dirancang agar tidak hilang meski server sempat bermasalah — tanpa menambah biaya infrastruktur di luar yang sudah dikuotasikan di atas.

---

## 3. Timeline Deliverable

Seluruh pekerjaan aplikasi (Modul 1–4) **sudah selesai** dan aplikasi sudah terpasang di server production (tarunaedu.com). Tahap yang tersisa adalah UAT dan go-live untuk pengguna.

| Fase | Deliverable | Status |
|---|---|---|
| **Fase 1 — Fondasi & Transaksi** | Discovery, design system, Auth 3 role, notifikasi email, shell dashboard admin + portal siswa, CRUD program, katalog, checkout 3 metode pembayaran, manajemen pesanan, enrollment.<br>**→ Modul 1 + 2: transaksi end-to-end.** | Selesai |
| **Fase 2 — Tryout Engine & Tes Kecermatan** | Bank soal, tryout builder (+ toggle acak soal/jawaban), pengerjaan tryout (timer + proctoring), scoring, hasil & ranking, live monitoring, builder + engine tes kecermatan, billing metered per-attempt.<br>**→ Modul 3 + 4: tryout & tes kecermatan dapat dikerjakan end-to-end.** | Selesai |
| **Fase 3 — UAT & Go-Live** | Load test 1.000 CCU, UAT bersama klien, bug fixing, go-live untuk pengguna (user production). | Berjalan — tanggal go-live disepakati bersama klien |

---

## 4. Termin Pembayaran

| Termin | Pemicu | Nilai | Porsi |
|---|---|---|---|
| Termin 1 | Seluruh pekerjaan aplikasi selesai 100% | Rp 30.000.000 | 57% |
| Termin 2 — Pelunasan | Setelah go-live user production | Rp 22.500.000 | 43% |
| | **Total** | **Rp 52.500.000** | 100% |

Hak kepemilikan source code & aset (IP) berpindah ke klien setelah **pelunasan penuh**.

---

<div style="page-break-before: always"></div>

## 5. Asumsi & Ketentuan

- Harga bersifat **net**; PPN (bila berlaku) belum termasuk.
- **Infrastruktur** (server VPS Hostinger + Cloudflare) ditagihkan **at-cost** sesuai tagihan penyedia, tanpa markup. Perpanjangan Tahun ke-2 dihitung terpisah.
- **1000 CCU** adalah beban puncak bersamaan (siswa mengerjakan tryout kapan saja, self-paced — bukan event serentak terjadwal). Beban di atas kapasitas tersebut memerlukan penyesuaian kapasitas (scope & biaya terpisah).
- **Maintenance** gratis 1 tahun mencakup perbaikan bug fitur dalam scope. Setelah masa garansi, retainer bulanan dihitung terpisah.
- Permintaan di luar scope tiap modul dihitung sebagai **change request** berbayar.
- Timeline bergantung pada kecepatan approval klien, penyediaan konten (soal, materi), dan ketersediaan akun pihak ketiga (Midtrans/Xendit).
- Penambahan atau penghapusan salah satu dari 3 metode pembayaran (Gateway/Manual/Saldo) setelah Fase 1 selesai dihitung sebagai change request.

---

## 6. Tidak Termasuk *(biaya pihak ketiga / opex klien)*

- Fee payment gateway Midtrans/Xendit (~0,7–2% per transaksi) — berlaku untuk transaksi yang menggunakan metode Payment Gateway.
- Biaya provider notifikasi email & WhatsApp (mis. Twilio/Fonnte), per pesan, di luar kuota gratis provider.
- Domain, SSL khusus, dan lisensi pihak ketiga lainnya.
- Migrasi soal dari Excel/Word existing.
- Konten platform (bank soal, pembahasan) — disediakan oleh klien.
- Aplikasi mobile native — di luar scope MVP.

---

**Document End**
