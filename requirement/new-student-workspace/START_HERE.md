# Prompt untuk agent penerima

Lanjutkan redesign student workspace Taruna Eduka di repo `bimbel-tryout`, berdasarkan paket handoff ini. Owner meminta handoff untuk pekerjaan berikutnya; jangan menganggap paket ini sebagai bukti implementasi sudah selesai atau izin merge/deploy.

1. Baca `AGENTS.md`, `web/AGENTS.md`, `requirement/new-student-workspace/HANDOFF.md` dan `requirement/new-student-workspace/PLAN.md`. Baca sumber requirement/domain yang ditunjuk sebelum mengubah perilaku.
2. Refresh status Git dan main, periksa diff lokal. Saat audit, branch `codex/student-dashboard-full-width` memiliki dua perubahan full-width belum di-commit; salinannya ada di `reference/full-width.patch`. Jangan reset pekerjaan ini atau apply patch dua kali. Gunakan branch/worktree terisolasi bila mulai implementasi.
3. Jalankan prototype yang dibundel sesuai HANDOFF; review desktop/mobile dan kedua runtimes. Prototype adalah referensi visual dan flow, bukan implementasi otoritatif.
4. Ikuti paket berurutan, satu slice/batas review setiap kali. Prioritaskan Tryout dan Kecermatan. Gunakan Go + Next/React/HeroUI existing, typed API dan token/i18n repo. Jangan memindahkan seluruh prototype menjadi aplikasi atau membawa scoring, saldo, timer, simulation control dan fake profile save ke production.
5. Ikuti instruksi eksekusi owner pada chat baru untuk batas pekerjaan. Palet sudah dipilih owner: **rose hangat bersama untuk siswa dan admin**; baca `PALETTE-DECISION.md`, jangan kembali memakai baseline biru lama atau menanyakan pilihan palet lagi. Gate profil mutation, checkout multiproduk dan D03 tetap tidak boleh diputuskan diam-diam. Kerjakan bagian independen yang sudah jelas, laporkan gate yang tersisa.
6. Buktikan browser → API → database untuk flow yang dikerjakan; pertahankan server deadlines, ACK, money idempotency, ownership, proof-before-create dan delayed review. Jangan mengklaim sandbox provider terbukti dari mock.
7. Perbarui DESIGN/domain docs/PLAN hanya sesuai perubahan dan bukti. Laporkan file berubah, checks, demo, risiko dan deferred. Jangan push/merge/deploy tanpa instruksi owner; commit tanpa atribusi agent.

Mulai dengan merangkum baseline checkout terkini, paket yang akan dikerjakan dan cara membuktikannya. Jangan meminta owner mengulang konteks yang sudah ada di paket.

Tambahan: owner juga menanyakan desain portal admin. Baca `ADMIN-PROPOSAL.md` di folder ini sebagai arah flow admin yang **belum disetujui untuk implementasi penuh**. Pilihan palet rose bersama sudah ditetapkan; hal itu tidak otomatis membuka scope modul/API admin. Prototype admin terpisah sekarang ikut dibundel di `prototype/admin/` untuk tiga journey review; baca README-nya. Rencana implementasi tetap berfokus siswa; jangan memperluas scope implementasi ke admin tanpa instruksi eksekusi owner.
