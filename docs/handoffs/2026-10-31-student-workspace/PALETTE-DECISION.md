# Keputusan palet bersama — rose hangat

**Keputusan owner, 31 Oktober 2026:** “kita coba pake rose hangat, update handoff docs nya”. Rose hangat menjadi arah bersama untuk dashboard siswa dan portal admin/operator. Halaman autentikasi terkait memakai fondasi yang sama. Keputusan ini menggantikan baseline biru dan gate pemilihan rose/blue dalam versi handoff sebelumnya.

**Status pekerjaan turn ini:** dokumentasi dan paket handoff diperbarui. Source CSS/JS prototype dan aplikasi tidak diubah; screenshot/admin preview putih-biru adalah bukti versi sebelumnya, bukan target warna final. Pilihan keluarga warna sudah jelas; penerapan dan review visual hasilnya masih harus dilakukan agent berikutnya dalam scope eksekusi yang diberikan owner.

## Target tampilan bersama

- Canvas krem hangat, kartu/header putih hangat, teks plum/coklat tua, aksen rose untuk aksi dan pilihan aktif.
- Sidebar desktop siswa dan admin menggunakan keluarga plum gelap dan teks terang yang sama. Ubah juga surface sidebar admin; mengganti tombol biru menjadi rose saja tidak menyelesaikan perbedaan dominan yang ditunjukkan owner.
- Terapkan konsisten pada katalog/cart, checkout/status pembayaran, wallet/topup, detail/result/review kedua runtime, serta tabel/editor/detail di portal admin sesuai paket yang dikerjakan. Dialog/drawer/focus/hover/disabled juga ikut token bersama.
- Struktur navigasi tetap sesuai peran: tiga tujuan utama untuk siswa; grup operasional/konten/keuangan/sistem untuk staf. Admin dapat lebih padat, siswa lebih lega. Penyamaan warna tidak menghapus perbedaan kebutuhan layout atau role guard.
- Success tetap hijau, pending/perhatian tetap amber, error/destructive tetap merah melalui semantic token dan label yang konsisten di kedua area. Rose adalah identitas visual, bukan pengganti seluruh status.
- Light/dark dan ID/EN tetap didukung. Dark menggunakan versi rose hangat yang sama antarkedua area, tidak kembali ke biru untuk admin.
- Logo Taruna Eduka mempertahankan aset/warna aslinya. Marketing/public landing tidak otomatis mendapat rebrand dari keputusan workspace ini; katalog signed-in dan auth yang terkait mengikuti workspace.

## Referensi token untuk implementer

Sumber angka di bawah adalah **palet rose yang sudah ada** pada `prototype/style.css`, selector `:root` dan `:root[data-theme="dark"]`. Ini baseline implementasi yang dapat ditelusuri, bukan klaim audit contrast atau persetujuan tiap nilai hex secara terpisah. Gunakan semantic CSS variables di aplikasi; jangan menyalin literal warna ke komponen.

| Tujuan semantic aplikasi | Referensi prototype | Light | Dark | Pemakaian |
|---|---|---|---|---|
| `background` | `--canvas` | `#f7f2ef` | `#1c191e` | Canvas krem hangat |
| `surface` | `--panel` | `#fffdfa` | `#28242b` | Kartu, form dan header |
| `foreground` | `--ink` | `#3e3239` | `#f6edf2` | Teks utama |
| `muted` | `--muted` | `#786771` | `#b8aab4` | Teks pendukung |
| `accent` | `--accent` | `#6b455d` | `#e0b6cb` | CTA, link dan fokus |
| `accent-soft` | `--accent-soft` | `#eee1e6` | `#41323f` | Pilihan aktif dan area aksen lembut |
| `border / separator` | `--line` | `#e7dfe0` | `#443a44` | Pemisah dan batas komponen |
| `workspace-sidebar` | `--rail` | `#30272e` | `#151216` | Sidebar plum gelap |
| `workspace-sidebar-foreground` | `--rail-text` | `#e6dce1` | `#eee4eb` | Teks/icon pada sidebar |
| `workspace-rose` | `--rose` | `#9e8189` | `#b794a6` | Aksen pendukung |
| `workspace-peach` | `--peach` | `#e8d2c4` | `#59424f` | Ilustrasi/dekorasi pendukung |
| `ring-start / ring-end` | `--accent / --ring` | `#6b455d / #a87f94` | `#e0b6cb / #c292b0` | Progress ring satu keluarga warna |

Pasangan `accent-foreground`, warna status, field boundaries, focus dan disabled harus diselaraskan serta diuji dalam light/dark sesuai DESIGN. Jangan memakai foreground gelap/terang yang sama secara membuta saat accent berganti pada dark mode. Nama token khusus sidebar di tabel adalah usulan pemetaan; reuse token existing bila sudah cocok, tanpa membuat theme engine baru.

## Urutan pekerjaan agent penerima

1. Baca keputusan ini sebelum membandingkan screenshot. Preview siswa bisa dipilih rose/blue, sedangkan admin yang dibundel masih putih/biru; keduanya adalah referensi flow. **Rose adalah acuan pekerjaan berikutnya**, pilihan blue demo bukan alternatif keputusan yang masih terbuka.
2. Petakan semantic token existing ke referensi rose dan terapkan fondasi yang sama pada auth, siswa dan shell admin. Update `web/app/globals.css`, komponen terkait dan `requirement/DESIGN.md` bersama saat implementasi. Hindari efek rebrand tak sengaja pada marketing; pakai scope workspace bila perlu.
3. Pertahankan layout/flow yang sudah direview. Penerapan palet bersama pada shell admin tidak berarti izin membangun seluruh modul admin atau menambah API/policy.
4. Ambil screenshot siswa dan admin pada viewport, locale dan mode yang sama. Bandingkan background, sidebar, kartu, teks, CTA, ring, tabel, form serta state. Minta review hasil rose; tidak perlu menanyakan kembali pilihan rose vs biru.
5. Catat contrast, keyboard/focus, reduced motion, status readability dan viewport checks sesuai DESIGN §10. Perbarui screenshot/handoff dengan label versi yang benar; jangan menandai screenshot biru lama sebagai bukti penerapan rose.

## Acceptance penyelarasan

- [ ] Siswa dan admin dalam light mode menunjukkan canvas krem, sidebar plum dan aksen rose dari sumber token yang sama; tidak ada area yang masih dominan putih/biru karena token lama.
- [ ] Auth terkait, menu akun, drawer/dialog, katalog/cart/checkout/topup serta halaman ujian/hasil mengikuti fondasi yang sama sesuai scope implementasi.
- [ ] Dark mode konsisten di kedua area dan pasangan foreground/accent terbaca; status success/pending/error tetap dapat dibedakan beserta labelnya.
- [ ] Layout, kepadatan konten, hak akses, timer/scoring/money dan flow bisnis tetap mengikuti spesifikasi masing-masing.
- [ ] Logo asli dan marketing di luar scope tetap benar; DESIGN/token docs dan screenshot terbaru sinkron dengan hasil.

Keputusan ini menutup **pemilihan keluarga palet**. Gate profil mutation, combined checkout, D03, provider sandbox dan scope implementasi admin tetap berlaku sebagaimana HANDOFF/ADMIN-PROPOSAL.
