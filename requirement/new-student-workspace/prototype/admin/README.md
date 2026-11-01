# Preview portal admin/operator
## Keputusan palet terbaru

Owner memilih **rose hangat bersama untuk siswa dan admin** pada 31 Oktober 2026. Baca [PALETTE-DECISION.md](../../PALETTE-DECISION.md). Source/screenshot prototype tetap versi review sebelumnya: admin masih putih/biru; pilihan blue pada demo siswa hanya alternatif historis. Target handoff berikutnya adalah rose bersama, dengan sidebar plum, canvas krem dan state konsisten. Update dokumen ini tidak menerapkan CSS atau mengklaim visual rose admin sudah diverifikasi.



Preview lokal yang dapat diklik untuk review visual dan flow, bukan aplikasi production. Tidak ada API, login, bank, provider atau sesi ujian asli yang dipanggil. Semua orang, proof, angka, waktu dan keputusan adalah data contoh. Penyimpanan hanya `sessionStorage` key `taruna-admin-preview-v1`.

## Menjalankan

Dari root repo:

```bash
python3 -m http.server 4182 --bind 127.0.0.1 --directory requirement/new-student-workspace/prototype
```

Buka `http://localhost:4182/admin/#/dashboard`. Jika memakai server bundle pada port 4181, buka `/admin/` pada server tersebut. Prototype siswa tetap di `/flow/`.

## Alur review

1. **Ringkasan → Monitoring → Detail Naila → Tambah waktu.** Form memvalidasi 1–240 menit dan alasan. Hasil contoh memperbarui deadline, total extension dan timeline. Sesi asli tidak berubah.
2. **Tes kecermatan → Edit konfigurasi → Pratinjau kolom → Simpan tes contoh.** Kolom genap 10–50, soal 50–100, durasi bilangan bulat minimal 1. Draf tetap tersedia setelah reload; ringkasan mengikuti konfigurasi. Preview kolom adalah ilustrasi simbol statis, tidak mengimplementasikan generator/scoring server. Item image/emoji/mix berada di tahap lanjutan.
3. **Pesanan → TE-1048 → Bukti contoh → Konfirmasi / Batalkan.** Keputusan memakai halaman terpisah, acknowledgement untuk konfirmasi dan alasan untuk pembatalan. Checkbox gagal simpan sekali membiarkan status pending; retry kemudian mengubah entity contoh yang sama. Gateway TE-1045 tidak punya tombol manual confirm.
4. Di footer, ubah **Admin/Operator**, **Normal/Data tidak mutakhir/Gagal memuat/Data kosong**, atau reset demo. Program, akun staf dan settings hilang untuk Operator. ID/EN dan light/dark ada di header; rail bisa diciutkan, mobile memakai drawer.

Halaman lanjutan (Tryout builder, siswa, hasil, topup, import, sistem) diberi penjelasan belum masuk preview. Tidak ada tombol yang mengklaim kemampuan backend baru. Dialog konten tidak digunakan pada tiga journey ini.

## Verifikasi 31 Oktober 2026

- Syntax `admin.js` dan `copy.js`: pass.
- Browser: extension +15 menit mengubah contoh deadline 11.05→11.20 dan timeline; tidak ada perubahan server.
- Builder: 11 kolom ditolak; 12 kolom ×50 menghasilkan ringkasan 600; program CASN dan konfigurasi bertahan setelah reload; preview dan save contoh berjalan.
- Pesanan: simulated save failure tidak mengubah pending; retry confirmation berhasil; pembatalan tanpa alasan ditolak lalu alasan tersimpan; gateway tidak bisa manual-confirm.
- Role Operator menyembunyikan Program/Akun staf/Settings. English + dark/mobile diperiksa. Stale/error/retry/empty dan drawer mobile dijalankan di browser.
- Sembilan halaman ×320/390/768/1024/1440/1920/2560px =63 pemeriksaan tanpa page-level horizontal overflow. Tabel monitoring/pesanan dapat scroll internal. Dashboard mobile merangkum subtes/pelanggaran pada baris siswa agar aksi detail tetap terlihat.
- Browser console errors: tidak ditemukan pada akhir pemeriksaan. Screenshot dan `admin-layout-checks.json` ada di `../../reference/`.

Batas: belum audit contrast/a11y penuh, race API, provider sandbox, timer/server authority, real upload atau backend integration. Pergantian role hanya kontrol preview, bukan authorization production. Proposal dan gap kontrak tetap berlaku di [ADMIN-PROPOSAL.md](../../ADMIN-PROPOSAL.md). Build aplikasi dan tes Go tidak dijalankan untuk prototype standalone ini.
