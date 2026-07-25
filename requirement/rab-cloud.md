# RAB Infrastructure Cloud — Platform Tryout Online (Taruna Eduka)

| |                                                                                                                           |
|---|---------------------------------------------------------------------------------------------------------------------------|
| **Versi** | 1.1                                                                                                                       |
| **Tanggal** | 5 Juli 2026                                                                                                               |
| **Dibuat oleh** | Saifullah Panca                                                                                                           |
| **Referensi** | `PRD.md`, `RAB.md` (Infrastruktur Cloud)                                                         |
| **Catatan** | Dokumen terpisah dari `RAB.md` utama — analisa pendukung, detail option infrastruktur bagian dari penawaran resmi ke klien |

---

## 1. Asumsi Arsitektur

Stack sesuai PRD — **satu VM lean**, bukan setup multi-VM/managed-services penuh:

| Komponen | Kebutuhan | Catatan |
|---|---|---|
| **API Server** | Go (Echo v4) | Satu VM, tidak perlu horizontal scaling di skala ini |
| **Database** | PostgreSQL | Self-hosted di VM yang sama, bukan managed (Cloud SQL/RDS) |
| **Cache/Queue** | Redis + River (job queue Postgres-backed) | Self-hosted, satu VM yang sama — lihat PRD §7.1 |
| **Object Storage** | GCS/S3-compatible | Gambar soal + bukti transfer, volume kecil |
| **CDN** | Cloudflare (free tier) | Aset statis (JS/CSS/gambar) saja — **bukan** konten ujian (lihat PRD §7.2, alasan randomisasi) |
| **Load Balancer** | Tidak perlu | Satu VM, tidak ada multi-instance |

## 2. Asumsi Skala

Ada 2 skenario yang perlu dibedakan secara eksplisit di project ini:

### Skenario A — Self-Paced Normal (baseline sehari-hari)

| Parameter | Nilai                                                                                    |
|---|------------------------------------------------------------------------------------------|
| Siswa terdaftar | 10.000                                                                                   |
| Target peak CCU | **1.000** — tersebar sepanjang hari, siswa mengerjakan kapan saja (bukan event serentak) |
| Pola beban | Rendah dan tidak menentu waktunya — tidak ada lonjakan bersamaan                         |

### Skenario B — Event Tryout Serentak (burst, worst-case)

| Parameter | Nilai                                                                                                                                                  |
|---|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Jumlah siswa serentak** | **1.000 siswa** memulai/submit dalam window waktu sempit (event tryout terjadwal, bukan self-paced)                                                    |
| Burst read (mulai subtes bersamaan) | ~100-200 concurrent                                                                                                                                    |
| Steady-state write (saat event) | ~10-15/detik                                                                                                                                           |
| Status | Skenario nyata yang disebut klien sebagai ketika terdapat event tryout sekolah — **inilah dasar buffer 2x** di RAB (Infrastruktur Cloud) |

Sizing VM di bawah mengasumsikan Skenario B (worst-case) sebagai batas atas, bukan Skenario A — supaya tidak under-provision kalau event serentak ini benar-benar terjadi.

### Validasi Bottleneck terhadap Skenario B

Dicek per komponen, bukan asumsi kasar — 2 vCPU / 4 GB RAM tetap cukup **dengan satu syarat implementasi**:

| Layer | Beban di Skenario B | Cukup di 2vCPU/4GB? |
|---|---|---|
| **Go API compute** | ~100-200 concurrent request | **Ya, jauh dari batas.** Goroutine per-request murah (KB-level), mayoritas waktu request menunggu I/O (query DB), bukan CPU-bound. 2 vCPU cukup untuk JSON marshal/unmarshal + business logic di volume ini. |
| **PostgreSQL — write throughput** | ~10-15 write/detik | **Ya, jauh dari batas.** Postgres modern menangani ratusan write/detik untuk operasi OLTP sederhana di hardware sekecil ini. |
| **PostgreSQL — koneksi** | Hingga 200 request bersamaan | **Butuh connection pooling di sisi app (pgx pool, ~15-25 koneksi), bukan 1 koneksi per-request.** Tanpa ini, 200 koneksi mentah × ~5-10MB overhead/koneksi = 1-2GB RAM habis cuma buat overhead koneksi — bisa bikin OOM di mesin 4GB. Dengan pool kecil di app (sudah standar practice pakai `pgx/v5`, sesuai PRD §7), risiko ini hilang tanpa perlu PgBouncer terpisah atau VM lebih besar. |
| **Redis** | Sorted set ranking + cache subtes | **Ya, sangat ringan.** Volume data kecil (10rb user, question bank terbatas), operasi Redis cepat, pemakaian CPU/RAM minimal. |

**Kesimpulan:** 2 vCPU / 4 GB RAM (e2-medium atau setara) tetap valid untuk Skenario B — **bukan** karena bebannya cukup saja, tapi karena sudah dicek per-layer dan gak ada yang mendekati batas, dengan syarat pooling koneksi Postgres dikonfigurasi benar di app (item implementasi, bukan tambahan biaya infra).

---

## 3. Perbandingan Provider — Rincian per Komponen (VM 2 vCPU / 4 GB RAM, setara e2-medium)

Setiap provider dipecah per komponen biaya, bukan cuma angka VM-nya, supaya jelas sumber cost-nya dan bisa dibandingkan apple-to-apple.

**Asumsi volume shared** (sama untuk semua provider, basis per bulan):
- Disk VM: **50GB** (OS + data Postgres + Redis) — pd-balanced / equivalent
- Object storage: **10GB** (gambar soal + bukti transfer)
- Bandwidth egress: **50-100GB** (JSON API + aset dinamis, **setelah** Cloudflare cache aset statis di depan)
- Backup: snapshot disk **~50GB**
- Monitoring/Logging: volume kecil di skala ini (mayoritas free tier)

| Komponen | GCP (asia-southeast2)                | AWS (ap-southeast-3)               | Alibaba (ap-southeast-5)          | Biznet Gio (ID)                                                       |
|---|--------------------------------------|------------------------------------|-----------------------------------|-----------------------------------------------------------------------|
| **VM compute** | e2-medium (**~$28-29**) (est)        | t3.medium **~$38-42** (est)        | ECS 2vCPU/4GB **~$28-32** (est)   | NEO Lite **~$8,5** (confirmed)                                        |
| **Disk (OS+data)** | pd-balanced 50GB (**~$0,13/GB**)     | EBS gp3 50GB **~$5-6** (est)       | ESSD PL1 50GB **~$3,5-4,5** (est) | included / add-on murah **~$0-2** (perlu verifikasi included storage) |
| **Object storage** | GCS Standard 10GB (**~$0,020/GB**)   | S3 Standard 10GB (**~$0,023/GB**)  | OSS Standard 10GB (**~$0,018/GB**) | add-on **~$1-2** (est)                                                |
| **Bandwidth egress** | 50-100GB (**~$0,105/GB**)            | 50-100GB (**~$0,114/GB**)          | 50-100GB (**~$0,050-0,082/GB**)       | diklaim **gratis** (perlu verifikasi fair-use cap)                    |
| **Monitoring/Logging** | Cloud Monitoring free tier **~$0-2** | CloudWatch free tier **~$0-3**     | CloudMonitor free tier **~$0-2**  | dasar included **~$0**                                                |
| **Backup (snapshot)** | snapshot 50GB (**~$0,026/GB**)       | EBS snapshot 50GB  (**~$0,05/GB**) | snapshot 50GB (**~$0,03-0,05/GB**) | manual / add-on **~$0-1**                                             |
| **Total/bulan** | **~$41-49**                          | **~$51-65**                        | **~$36-49**                       | **~$10-14**                                                           |

**Catatan per komponen:**
- **VM** — base harga us-region + premium regional Jakarta.
  - GCP base $24,46 (us-central1) + ~15-20% premium Asia.
  - AWS base $30,37 (us-east-1) + ~30-40% premium Jakarta (konsisten dengan temuan RAB Akademi Bimbel).
  - Alibaba "hampir setara" GCP di lean setup per referensi yang sama.
  - Biznet Gio harga listed langsung dari biznetgio.com/pricelist (Rp139.000).
- **Disk** — rate pd-balanced GCP Jakarta **confirmed** $0,13/GB.
  - AWS/Alibaba ekstrapolasi dari base tier + sedikit premium regional.
  - Biznet Gio biasanya bundle storage dalam paket NEO Lite, perlu cek included volumenya.
- **Object storage** — volume kecil (gambar soal + bukti transfer), biaya nyaris tidak signifikan di skala ini. Rate antar provider beda tipis.
- **Bandwidth egress** — komponen paling sulit diestimasi tanpa data traffic real.
  - Asumsi 50-100GB untuk 1.000 CCU self-paced (JSON API + aset dinamis).
  - aset statis sudah di-cache Cloudflare (free tier) jadi gak kenal egress origin.
  - Sisi bawah (\~50GB) untuk engagement normal, sisi atas (\~100GB) kalau engagement tinggi — perlu monitor setelah launch dan sesuaikan.
  - Biznet Gio diklaim "gratis hingga 10 Gbps" tapi fair-use cap belum diverifikasi.
- **Monitoring/Logging** — di skala 1.000 CCU, free tier tiap provider umumnya cukup. Akan mulai kena charge kalau retention log panjang atau metric volume tinggi.
- **Backup** — snapshot disk untuk DR. Alternatif lebih murah: `pg_dump` cron → GCS/OSS (~$1-2/bulan untuk storage 10-50GB), tapi bukan snapshot block-level.

**Confidence level per komponen:**
- **Confirmed langsung dari sumber resmi:** VM Biznet Gio, Disk pd-balanced GCP rate Jakarta.
- **Estimasi/ekstrapolasi:** Semua perhitungan diatas bersifat estimasi/eksplorasi ambil harga base region US dan di kalkulasikan dengan permium region asia, butuh divalidasi ke kalkulator resmi ketika akan dipakai.

## 4. Ringkasan Total Infra/Bulan

Total di bawah adalah jumlah dari rincian per komponen dipoin 3. Untuk breakdown dan confidence level per komponen.

| Provider | Total estimasi/bulan | Catatan ringkas |
|---|---|---|
| **GCP** | **~$41-49** | Default asumsi PRD/RAB; disk pd-balanced confirmed; tooling (Logging/Monitoring) matang |
| **AWS** | **~$51-65** | Premium Jakarta paling tinggi; CloudFront lebih mahal dari Cloud CDN per pola Akademi Bimbel |
| **Alibaba** | **~$36-49** | CDN/disk paling murah; sedikit lebih hemat dari GCP di komponen egress |
| **Biznet Gio** | **~$10-14** | Jauh lebih murah; bandwidth diklaim "gratis hingga 10 Gbps" (perlu verifikasi fair-use cap) |

**Catatan confidence level:** hanya harga VM Biznet Gio dan disk pd-balanced GCP yang confirmed langsung dari sumber resmi. Sisanya ekstrapolasi dari baseline harga + pola premium regional — validasi ke kalkulator resmi tiap provider sebelum dipakai untuk keputusan final/commit.

---

## 5. Kenapa Biznet Gio Layak Dipertimbangkan

Untuk project ini, Biznet Gio layak dipertimbangkan karena beberapa alasan:
- Kita sudah memutuskan self-host Postgres + Redis di satu VM yang sama untuk semua opsi provider (bukan cuma Biznet Gio) — jadi "tidak ada managed DB/Redis" bukan kekurangan Biznet Gio secara relatif, semua provider di skala ini kita treat sama.
- Skala 1.000 CCU self-paced jauh di bawah titik di mana auto-scaling native jadi krusial.

**Risiko yang tetap perlu diverifikasi sebelum pilih Biznet Gio:**
- Fair-use cap di balik klaim "bandwidth gratis hingga 10 Gbps" — perlu dicek term of service-nya.
- Track record SLA/reliability dibanding GCP (99,95% SLA) — Biznet Gio tidak mempublikasikan SLA setara di hasil pencarian ini.
- Support & dokumentasi lebih terbatas dibanding GCP/AWS untuk tim yang mungkin belum familiar.

---

## 6. Rekomendasi

| Prioritas | Pilihan | Justifikasi |
|---|---|---|
| **Default (konsisten dengan RAB/PRD saat ini)** | **GCP Jakarta, e2-small/medium** | Sudah jadi asumsi di PRD §7 dan RAB — tooling (Logging, Monitoring) matang, dokumentasi lengkap, tim kemungkinan lebih familiar |
| **Kalau biaya jadi prioritas utama** | **Biznet Gio** | ~$30-35/bulan lebih murah dari GCP — worth dicoba di tahap dev/staging dulu untuk validasi reliability sebelum commit ke production |
| **Tidak disarankan di skala ini** | AWS | Premium Jakarta paling tinggi tanpa manfaat tambahan yang relevan buat skala 1.000 CCU |

**Keputusan provider belum final** — ini analisa pendukung, bukan keputusan yang sudah dikunci. GCP tetap default karena sudah jadi asumsi di seluruh PRD/RAB saat ini; ganti provider berarti perlu update kedua dokumen itu juga.

---

*Estimasi berdasarkan riset harga Juli 2026 (WebSearch + WebFetch ke cloudprice.net, economize.cloud, biznetgio.com). Validasi di kalkulator resmi provider sebelum commit ke klien.*
