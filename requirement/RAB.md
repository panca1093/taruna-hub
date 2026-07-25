# RAB — Platform Tryout Online

| | |
|---|---|
| **Versi** | v1.3 |
| **Tanggal** | 3 Juli 2026 |
| **Berlaku s/d** | 2 Agustus 2026 (30 hari) |
| **Klien** | Taruna Eduka |
| **Lingkup** | Platform tryout self-paced (web) — SD/SMP/SMA/CASN/BUMN/AKPOL, ditambah format Tes Kecermatan |

> Notes feedback dengan klien: 
> - **3 metode aktif bersamaan** — Payment Gateway (Midtrans), Konfirmasi Manual, dan Saldo/Wallet — siswa memilih saat checkout (bukan pilihan eksklusif). 
> - RAB ini mengkuotasikan scope penuh ketiga metode tersebut. 
> - Format tes baru (Tes Kecermatan) dan randomisasi soal/jawaban pada tryout (Modul 3). 
> - Timeline dan harga pada version ini disesuaikan langsung oleh tim internal berdasarkan kapasitas riil, agar tetap pas dengan target go-live 3 Agustus 2026.

---

## 1. Ringkasan Biaya

| | 3 Metode Pembayaran + Tes Kecermatan |
|---|---|
| Subtotal Development (web) | **Rp 57.000.000** (sekali bayar) |
| Infrastruktur Cloud (Tahun 1) | **$50/bln ($600/thn)** — at-cost |
| Maintenance (gratis 6 bln) | Rp 0 |
| **TOTAL YEAR 1** | **Rp 57 jt + $600** (≈ Rp 67.8 jt @ kurs Rp 18.000) |

---

## 2. Rincian per Modul

| No | Modul | Cakupan / Deliverable                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Estimasi | Catatan |
|---|---|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---|---|
| 1 | **Discovery, Auth & Fondasi** | <ul><li>Requirement gathering & design system</li><li>Arsitektur & schema DB</li><li>Auth JWT 3 role (Student/Operator/Admin) + notifikasi email & WhatsApp</li><li>Shell dashboard admin + portal siswa, light/dark mode</li><li>Landing page marketing (halaman promosi di domain utama, terpisah dari aplikasi tryout — lihat PRD §5.19)</li><li>CRUD Program SD/SMP/SMA/CASN/BUMN/AKPOL</li></ul>                                                                                                                                                                                                          | Rp 7.500.000 | Infrastruktur dasar aplikasi, termasuk landing page |
| 2 | **Katalog & Transaksi (3 Metode Pembayaran: Midtrans/Manual/Saldo)** | <ul><li>Katalog & detail produk</li><li>Checkout dengan 3 metode aktif bersamaan:<ul><li>Payment Gateway Midtrans/Xendit dengan konfirmasi otomatis</li><li>Konfirmasi Manual dengan upload bukti transfer</li><li>Saldo/Wallet dengan potong saldo otomatis</li></ul></li><li>Riwayat pesanan siswa</li><li>Admin CRUD produk & manajemen pesanan</li></ul>                                          | Rp 13.500.000 | Integrasi payment gateway pihak ketiga, alur konfirmasi manual, dan saldo/wallet |
| 3 | **Tryout Engine** | <ul><li>Bank soal 4 tipe + LaTeX/MathJax + import massal</li><li>Tryout builder, termasuk toggle acak urutan soal & jawaban PG</li><li>Pengerjaan tryout: timer, navigasi, blocking, auto-submit, resume</li><li>Mode layar penuh wajib saat ujian + pencatatan pelanggaran</li><li>Scoring weighted + IRT</li><li>Hasil, ranking & pembahasan</li><li>Live monitoring</li><li>Dashboard hasil admin</li></ul> | Rp 28.500.000 | Modul paling kritikal |
| 4 | **Tes Kecermatan (Format Baru)** | <ul><li>Builder tes kecermatan: tipe item, panjang item, opsi/soal, soal/kolom, jumlah kolom, timer</li><li>Engine pengerjaan: generate set referensi per kolom, soal acak-hilang-1, jeda antar-kolom, auto-kosong saat timer habis</li><li>Billing metered per-attempt: beli attempt, potong kuota, beli tambahan</li><li>Hasil B/S/K + 3 skor komposit (Kecepatan/Ketahanan/Ketelitian) + pembahasan</li><li>Dashboard hasil admin</li></ul> | Rp 7.500.000 | Produk & billing terpisah dari tryout; formula skor komposit masih perlu validasi klien |
| | **Subtotal Development** |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | **Rp 57.000.000** | |
| 5 | **Infrastruktur Cloud (Tahun 1)** | <ul><li>Compute VM GCP Jakarta e2-small/medium (API + PostgreSQL + Redis)</li><li>Object storage, monitoring & logging, bandwidth</li></ul>                                                                                                                                                                                                                                                                                                                                                                                 | **$50/bln ($600/thn)** | Dikuotasikan USD; at-cost ke akun cloud klien; termasuk buffer 2x untuk skenario tryout event dengan peak 1.000 siswa serentak |
| 6 | **Maintenance** | <ul><li>Bug fix fitur dalam scope</li></ul>                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Rp 0 | Gratis 6 bulan; setelahnya retainer terpisah |
| | **TOTAL YEAR 1** |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | **Rp 57 jt + $600** | |

> Catatan strategi teknis (detail di PRD §7.1):
> - Notifikasi (email/WhatsApp) dan data transaksi dirancang agar tidak hilang meski server sempat bermasalah — tanpa menambah biaya infrastruktur di luar yang sudah dikuotasikan di atas.
> - Angka final tetap menunggu hasil load test 1.000 CCU (Fase 3, lihat §3).

---

## 3. Timeline Deliverable

Estimasi total **±5 minggu** — development selesai akhir Juli, UAT tersedia mulai 30 Juli, go-live 3 Agustus, konsisten dengan target go-live PRD.

| Fase | Deliverable | Minggu |
|---|---|---|
| **Fase 1 — Fondasi & Transaksi** | Discovery, design system, arsitektur & schema DB, Auth JWT 3 role, notifikasi email & WhatsApp, shell dashboard admin + portal siswa, CRUD program, katalog, checkout 3 metode pembayaran, manajemen pesanan, enrollment.<br>**→ Serah terima Modul 1 + 2: transaksi end-to-end.** | Minggu 1–2 |
| **Fase 2 — Tryout Engine & Tes Kecermatan** | Bank soal, tryout builder (+ toggle acak soal/jawaban), pengerjaan tryout (timer + proctoring), scoring, hasil & ranking, live monitoring, builder + engine tes kecermatan, billing metered per-attempt.<br>**→ Serah terima Modul 3 + 4: tryout & tes kecermatan dapat dikerjakan end-to-end.** | Minggu 3–4 (akhir Juli) |
| **Fase 3 — UAT & Go-Live** | Load test 1.000 CCU, UAT bersama klien mulai 30 Juli, bug fixing, deploy production, go-live 3 Agustus. | Minggu 5 (30 Juli – 3 Agustus) |

---

## 4. Termin Pembayaran

| Termin | Pemicu | Porsi |
|---|---|---|
| Termin 1 — DP | Saat kontrak/mulai pengerjaan | 30% |
| Termin 2 — Pelunasan | Serah terima seluruh modul (go-live) | 70% |

Hak kepemilikan source code & aset (IP) berpindah ke klien setelah **pelunasan penuh**.

---

## 5. Asumsi & Ketentuan

- Harga bersifat **net**; PPN (bila berlaku) belum termasuk.
- **Infrastruktur** dikuotasikan dalam USD (referensi GCP Jakarta) karena cloud ditagih USD dan kurs fluktuatif. Biaya aktual ditagihkan **at-cost ke akun cloud milik klien**. Renewal Tahun ke-2 dihitung terpisah.
- **1000 CCU** adalah beban puncak bersamaan (siswa mengerjakan tryout kapan saja, self-paced — bukan event serentak terjadwal). Beban di atas kapasitas tersebut memerlukan penyesuaian kapasitas (scope & biaya terpisah).
- **Maintenance** gratis 6 bulan mencakup perbaikan bug fitur dalam scope. Setelah masa garansi, retainer bulanan dihitung terpisah.
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
