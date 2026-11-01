# Handoff package validation

Dijalankan 31 Oktober 2026 pada checkout `01c4a0203d1c15c4303a3c034217660514bf3d67` dengan local full-width diff yang disebut dalam HANDOFF.

- PASS: `node --check` untuk bundled student `flow.js`/`data.js` serta admin `admin.js`/`copy.js`.
- PASS: tautan Markdown lokal dalam handoff, plan dan index menunjuk file yang tersedia.
- PASS: referensi HTML/CSS/module prototype tersedia di bundle; file source prototype identik dengan versi review, kecuali README yang diperbarui untuk portable run.
- PASS: gambar referensi siswa/admin dan patch full-width tersedia; manifest SHA-256 mencatat payload dan plan.
- PASS: proposal admin ditinjau satu agent melalui builder check; batas tiga journey prototype pertama diperjelas setelah review.
- PASS: struktur dan CRC ZIP diperiksa; source, assets, dokumen dan rencana ikut terpaket.
- PASS: `git diff --check`; dua perubahan aplikasi/DESIGN yang sudah ada dipertahankan.

Tidak dijalankan ulang: Go tests, Next typecheck/lint/build, browser/API/DB journeys atau gateway sandbox. Perubahan handoff/proposal serta prototype admin hanya menyentuh dokumen dan preview standalone; hasil geometry/browser sebelumnya dijelaskan terpisah di HANDOFF dan README prototype. Agent implementer wajib memverifikasi ulang terhadap aplikasinya.

Cek integritas setelah menerima paket dari root checkout:

```bash
shasum -a 256 -c requirement/new-student-workspace/SHA256SUMS
```

Preview portabel mengikuti instruksi di HANDOFF. Berkas manifest tidak mencatat dirinya sendiri.

Tambahan admin: tiga journey simulasi, validation/reload, role, ID/EN, tema, keadaan gagal dan 63 geometry checks dijalankan di browser. Detail dan batasnya ada di `prototype/admin/README.md`; bukti geometry di `reference/admin-layout-checks.json`. Ini tidak memverifikasi integrasi admin production.

Update keputusan owner (31 Oktober 2026): rose hangat dipilih sebagai palet bersama siswa/admin dan dicatat di `PALETTE-DECISION.md`; baseline biru serta gate pemilihan palet pada handoff/plan digantikan. Perubahan ini hanya dokumen/README/manifest/ZIP; CSS/JS aplikasi maupun prototype tidak diubah dan screenshot tidak diregenerasi. Pemeriksaan link, konsistensi referensi token, diff whitespace dan integritas ZIP/hash diperbarui; tidak ada klaim browser/contrast baru untuk palet rose admin.
