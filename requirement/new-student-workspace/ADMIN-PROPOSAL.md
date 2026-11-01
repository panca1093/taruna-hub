# Portal admin/operator — proposal desain tambahan

31 Oktober 2026 · Status: proposal + prototype lokal untuk review owner; belum implementasi aplikasi. Tambahan ini tidak memperluas otomatis scope implementasi student workspace. Baseline audit HEAD `01c4a0203d1c15c4303a3c034217660514bf3d67`.

## Arah yang diusulkan

Satu keluarga visual dengan siswa: full width, sidebar yang dapat diciutkan, tipografi/warna/token yang sama, jarak yang rapi dan light/dark. Hierarki konten mengikuti pekerjaan staf: **pantau sesi, siapkan latihan, tangani transaksi**. Owner sudah memilih **rose hangat bersama siswa/admin** pada 31 Oktober 2026; rincian flow/kepadatan admin di dokumen ini tetap usulan untuk review.

Admin memakai lebih banyak tabel dan editor. Jangan memaksakan batas tiga menu siswa ke admin: kelompokkan navigasi yang ada agar tetap mudah ditemukan. Ikuti [keputusan rose hangat bersama](PALETTE-DECISION.md), termasuk sidebar plum gelap dan canvas krem; baseline biru sebelumnya digantikan; status warning/danger harus punya arti dan label. Animasi hanya menjelaskan buka/tutup, perubahan status dan tindakan user; tabel live tidak bergerak atau berkedip setiap poll.

Scope tahap berikut yang direkomendasikan adalah **prototype admin** untuk review: ringkasan → monitoring sesi normal → detail sesi; authoring Kecermatan → preview; pesanan manual → bukti → keputusan. Prototype admin terpisah sekarang tersedia di `prototype/admin/`; prototype siswa tetap di `prototype/flow/`. Jangan langsung mengimplementasikan admin dari dokumen proposal.

## Keputusan warna owner

Rose hangat adalah acuan bersama untuk siswa dan admin. Preview/screenshot admin di bawah masih versi putih/biru sebelum keputusan ini; ambil layout dan flow-nya, lalu terapkan token bersama dari [PALETTE-DECISION.md](PALETTE-DECISION.md). Source preview tidak diubah dalam pekerjaan update dokumen ini.

## Preview yang sudah tersedia

Entry lokal: `http://localhost:4182/admin/#/dashboard`. [Panduan menjalankan dan alur uji](prototype/admin/README.md). [Dashboard](reference/admin-dashboard.jpg), [detail sesi](reference/admin-session.jpg), [builder](reference/admin-kecermatan-builder.jpg), [pratinjau kolom](reference/admin-kecermatan-preview.jpg), [review pesanan](reference/admin-order-review.jpg), [mobile gelap](reference/admin-mobile-dark.jpg).

Preview mengimplementasikan tiga journey pertama dengan data browser contoh, ID/EN, light/dark dan pilihan role. Owner meminta melihat preview, bukan implementasi production. Hasil pemeriksaan ada di README; checklist review owner di bawah tetap menunggu penilaian owner, bukan otomatis disetujui karena browser checks lolos.

## Navigasi yang lebih teratur

| Grup | Tujuan | Route existing / catatan |
|---|---|---|
| Ringkasan | Dashboard operasional | `/admin` |
| Operasional | Monitoring, Siswa, Hasil | `/admin/monitoring`, `/admin/students`; Hasil memakai tab normal/Kecermatan dengan URL existing `/admin/results` dan `/admin/kecermatan-results` |
| Konten | Tryout, Tes kecermatan, Paket item, Bank soal, Produk, Program | `/admin/tryouts`, `/admin/kecermatan`, `/admin/item-packages`, `/admin/questions`, `/admin/products`, `/admin/programs` |
| Keuangan | Pesanan, Topup | `/admin/orders`, `/admin/topups` |
| Sistem | Akun pengguna, Pengaturan | `/admin/users`, `/admin/settings`; Admin-only |

Grup bisa diciutkan; grup yang berisi halaman aktif terbuka otomatis. Sidebar collapsed tetap memberi tooltip/accessibility label. Header berisi breadcrumb, judul halaman, satu aksi utama sesuai konteks, lalu menu akun/tema/bahasa. Mobile menggunakan navigation drawer berkelompok, bukan memadatkan semua menu menjadi bottom bar siswa. Daftar panjang tetap punya pencarian/filter di area kontennya; global search belum diusulkan karena belum ada kontraknya.

Program tetap Admin-only walaupun berada di grup Konten. D02 mengizinkan Operator mengelola konten/paket item, siswa dan adjustment wallet; manajemen akun operator/admin serta settings tetap Admin-only. UI mengikuti guard API, tidak menambah role/capability matrix. D17 berarti satu platform/tenant terkonfigurasi: jangan menambahkan tenant switcher, impersonation atau Platform Admin ke MVP.

## Gambaran dashboard

```text
┌──────────────────┬──────────────────────────────────────────────────────┐
│ Taruna Eduka     │ Ringkasan operasional                  Akun / Tema   │
│                  ├──────────────────────────────────────────────────────┤
│ Ringkasan        │ Sesi normal aktif  │ Pesanan manual │ Topup manual │
│                  │                    yang perlu dicek                  │
│ Operasional      ├─────────────────────────────────┬────────────────────┤
│   Monitoring     │ Sesi tryout normal berjalan     │ Perlu ditangani    │
│   Siswa          │ Siswa / Tryout / Subtes / Status│ Pesanan / Topup    │
│   Hasil          │ Pelanggaran / Detail sesi       │ Lihat daftar       │
│                  │ Waktu sinkronisasi + stale state│                    │
│ Konten           ├─────────────────────────────────┴────────────────────┤
│ Keuangan         │ Pintasan: Tryout · Tes kecermatan · Bank soal         │
│ Sistem           │ Ke hasil normal / hasil Kecermatan                    │
└──────────────────┴──────────────────────────────────────────────────────┘
```

Ini wireframe hierarki, bukan nilai aktual. Pilih informasi yang bisa ditindaklanjuti; jangan mengisi layar dengan grafik dekoratif atau angka pertumbuhan tanpa data. KPI wajib punya sumber dan definisi yang tepat. Total pending tidak otomatis berarti semuanya menunggu pemeriksaan manual: bedakan metode manual dari gateway. Jangan menghitung total global dari satu halaman data.

Monitoring normal menjadi panel utama, dengan waktu sinkronisasi, refresh, error/stale state dan tautan ke sesi tepat. Tidak menjanjikan data realtime tanpa jeda: implementasi monitoring existing melakukan polling 30 detik. Kegagalan poll mempertahankan data terakhir dengan label sudah tidak mutakhir, bukan menampilkan nol.

Revenue dan grafik harian baru ditampilkan jika tersedia agregasi server yang tepat: periode, confirmed vs pending dan akses role jelas. Aplikasi saat audit **belum mempunyai admin dashboard summary API**; dashboard `/admin` sekarang memakai `mock-data`. Untuk implementasi, gunakan data endpoint yang sudah tersedia hanya sejauh kontraknya mendukung, atau buka paket summary API terpisah sesuai PLAN P22. Jangan mengganti mock dengan total yang diambil dari page pertama atau membuat angka dummy terlihat live.

## Flow utama dan bentuk halaman

| Area | Journey yang perlu didesain | Bentuk interaksi |
|---|---|---|
| Monitoring normal | Daftar sesi → detail sesi/subtes → timeline pelanggaran → tambah waktu dengan alasan → status terbaru | Detail sesi full page; penambahan waktu singkat dapat dialog konfirmasi. Tidak ada tombol hentikan/reset sesi baru tanpa policy/API. |
| Tryout | Daftar/filter → editor identitas/jadwal → susun subtes dan soal → preview → simpan/publish sesuai capability existing | Editor full page, struktur bagian jelas; preview/lock frozen assignment dipertahankan. Jangan menambah publish workflow bila API tidak mendukungnya. |
| Kecermatan | Daftar tes → konfigurasi kolom/soal/durasi → pilih paket/revisi → preview kolom → simpan → daftar produk terkait | Full page builder; preview pada panel/halaman tersendiri. Render karakter, image, emoji, solid shape, mix sesuai dukungan existing. Locked/revision states tetap terlihat. |
| Paket item dan bank soal | Daftar → editor/import → pratinjau accepted/rejected → konfirmasi accepted set → hasil | Import panjang dan preview memakai halaman; tidak ada penyimpanan sebelum konfirmasi. Snapshot/revisi frozen tidak ditimpa. |
| Pesanan | Filter status/metode → detail → bukti transfer → confirm/cancel dengan alasan bila diminta API → status final | Detail full page, bukti berdampingan dengan identitas/nominal pada desktop; konfirmasi pendek boleh dialog. Gateway tidak mendapat tombol manual confirm. |
| Topup | Antrean → detail/bukti → approve/reject → alasan/status → ledger siswa | Sama pola pesanan tetapi enum/status topup tetap terpisah. Pending belum kredit saldo. |
| Payment exception | Pesanan ditahan → evidence/alasan → pilihan existing credit wallet atau catat refund eksternal → hasil keputusan | Pertahankan D07 dan guard API; catat refund bukan otomatis mengirim uang. UI held-topup masih gap terpisah. |
| Siswa | Cari → profil readonly/data nyata → enrollment/hasil/ledger yang tersedia → adjustment dengan alasan | Detail panjang full page; adjustment singkat bisa dialog terkonfirmasi. Jangan menjanjikan CRUD identitas dari UI yang belum ada endpoint. |
| Hasil | Tab normal/Kecermatan → filter → session tertentu → hasil/review sesuai izin | Detail panjang full page, selected session stabil saat refresh/back; formula Kecermatan versi1 tetap provisional D03. |
| Sistem | Settings → ubah bobot/minimum → simpan/field errors; akun pengguna sesuai capability | Form halaman biasa; hindari save palsu atau role mutation mock. |

Tidak semua dialog harus dihapus: konfirmasi singkat dan aman boleh dialog. Detail transaksi, editor, import, laporan dan timeline panjang adalah halaman yang bisa dibuka ulang. Tidak ada modal bertumpuk. Aksi keuangan tidak dapat dijalankan hanya dengan klik row; tampilkan pihak, nominal, metode, status terbaru dan akibat tindakan sebelum konfirmasi.

## Kesiapan aplikasi dan gap kontrak

| Temuan audit source | Implikasi untuk agent berikutnya |
|---|---|
| `web/app/admin/page.tsx` membaca mock students/orders/revenue/liveSessions | Dashboard perlu data contract atau penyederhanaan berdasarkan API nyata; jangan mengklaim metrik live dari redesign. |
| `web/app/admin/users/page.tsx` memakai mock admins/operators dan local state | Manajemen akun bukan selesai hanya karena didesain ulang. Mutations wajib paket kontrak/role/retention tersendiri; read-only/explicit unavailable sampai ada kontrak. |
| Monitoring existing menyediakan sesi **normal** aktif, timeline dan extension melalui `api.monitoring` | Jangan melabelinya monitor live Kecermatan. Live Kecermatan perlu discovery/API scope baru; hasil Kecermatan existing tetap tersedia. |
| Orders/topups/results/students banyak menampilkan detail di `FormModal` | Pindah ke detail page memerlukan safe direct-read endpoint dan ownership/role guard, bukan row yang kebetulan tersimpan di browser. |
| `api.adminOrders` / `api.adminTopups` saat ini list + mutation, tanpa typed `get(id)` staf; monitoring hanya list aktif + timeline | Sebelum URL detail yang reloadable, audit wire contract. Endpoint student tidak boleh dipakai sebagai staff get. Jika detail read belum ada, kontrak-first dalam paket terpisah; jangan fetch seluruh database lalu filter browser. |
| `api.kecermatanResults.get(id)` ada; normal staff results berupa list/leaderboard | Detail hasil normal staff perlu audit kontrak; jangan memakai student result endpoint sebagai pengganti permission staf. |
| Settings bobot/minimum sudah memakai API; student wallet adjustment dan review transaksi juga memakai API | Pertahankan money exact strings, idempotency, audit reason dan persisted-result snapshots; jangan menulis ulang aturan karena pindah layout. |
| D03, P14 sandbox, P20/P21 exports serta P22 masih punya gate/deferred masing-masing | Tidak menampilkan export/analytics/bulk action seolah berfungsi jika slice belum tersedia. Tidak ada sign-off kebijakan baru lewat desain. |

Source utama: `web/lib/nav.ts`, `web/components/app-shell.tsx`, `web/app/admin/`, `web/lib/api/client.ts`, `api/openapi.yaml`, `requirement/DESIGN.md`, `ARCHITECTURE.md` D02/D03/D07/D17 dan status `requirement/PLAN.md`. Audit ini read-only; tidak mengubah role, API, mock management atau desain production.

## Batas prototype pertama

Paket pertama yang direkomendasikan hanya tiga journey: (1) ringkasan → monitoring normal → detail/extension, (2) builder Kecermatan → preview/simpan simulasi, (3) pesanan manual → bukti → keputusan simulasi. Semua memakai data contoh berlabel, termasuk skenario gagal. Pilihan role, tema, bahasa dan lebar layar diterapkan pada tiga journey ini.

Builder Tryout, topup, import, siswa/ledger, hasil dan sistem pada tabel di atas adalah arah cakupan lanjutan, bukan acceptance prototype pertama. Kerjakan sebagai paket review berikutnya jika owner memilih melanjutkan. Tidak ada code admin production dalam tahap prototype.

## Kriteria review prototype pertama

- [ ] Alur ringkasan → monitoring → detail sesi → tambah waktu dan hasilnya bisa ditelusuri; sesi normal dan Kecermatan tidak tercampur.
- [ ] Flow builder Kecermatan sampai preview/simpan simulasi terlihat, termasuk revisi/lock dan validation errors.
- [ ] Pesanan manual punya flow detail/bukti/keputusan, pending/confirmed/cancelled dan error; gateway ditandai tidak bisa dikonfirmasi manual. Held/topup lengkap menjadi paket lanjutan.
- [ ] Role Operator/Admin terwakili dalam shell dan tiga journey awal; program, management akun staf dan settings tidak bocor ke Operator. Tujuan di luar prototype diberi penanda belum tersedia, tidak berupa tombol mati tanpa penjelasan. Tidak ada tenant/platform-admin controls.
- [ ] Warna admin diselaraskan ke rose hangat siswa, termasuk canvas/sidebar/kartu/teks/tombol; bukti screenshot memakai versi rose terbaru.
- [ ] Tampilan 320/390/768/1024/1440/1920/2560px, light/dark, ID/EN, keyboard, focus, reduced motion dan long rows diperiksa. Table scroll internal, bukan horizontal overflow satu halaman.
- [ ] Polling tidak menggeser posisi row/focus diam-diam; stale/error/empty states jelas dan filter/pagination/back tetap konsisten.
- [ ] Semua data contoh prototype berlabel, tidak diklaim live; tidak ada credential/OTP/bukti transaksi asli.
- [ ] Owner review proposal/prototype sebelum dibuat rencana implementasi admin. Eksekusi siswa dan admin tetap paket terpisah; instruksi handoff tidak memberi izin push/merge/deploy.

## Batas keputusan

Rekomendasi penyusun: mulai prototype dari **monitoring + builder Kecermatan + review transaksi**, dengan ringkasan sebagai pintu masuk. Pemilihan prioritas ini bukan keputusan owner. Owner dapat mengubah prioritas saat review. Pemilihan palet sudah selesai: rose hangat bersama mengikuti PALETTE-DECISION. Penerapan dan review visualnya belum dikerjakan; screenshot biru lama bukan acuan warna final. Kemampuan yang memerlukan kontrak baru tetap deferred; developer bebas menentukan ekstraksi komponen/pola routing paling kecil yang sesuai repo, tanpa mengubah policy atau mengasumsikan admin redesign sudah disetujui.
