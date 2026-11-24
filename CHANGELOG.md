# Changelog

Notable changes to the Taruna Eduka tryout platform, newest first. Each version is a git tag; pushing a `v*` tag builds the deploy images.

## v0.1.7 — 2026-11-24

### Added

- **Packages can mix tryouts and kecermatan tests.** A bundle holds 2 to 20 items of either kind. Each purchase adds the product's attempts to every tryout and every kecermatan test in it, and the catalog lists both with the total time.
- **Search and filter on every list page.** Tryout, Tes kecermatan, Paket item, Program, Produk, Pesanan, Top-up, Monitoring, Siswa, Pengguna, Hasil and Hasil Kecermatan have the same search field and Filter button as Bank Soal. Students can filter Pesanan saya by status and Riwayat dompet by type.
- **Order details list the kecermatan tests of a mixed package,** next to its tryouts, so students see everything a purchase gave them.

### Changed

- **Choosing what a product sells.** The product form lists the chosen items and opens a separate window with a search box to pick them, instead of a long list or a dropdown. The bundle tab no longer shows a "0x" count.
- On Pesanan and Top-up the status tabs moved into the Filter panel.
- The catalog no longer says a tryout can be retaken any number of times; it says every attempt is saved as its own result.

> **Deploy note:** run migration `000034_bundle_kecermatan_tests`.

## v0.1.6 — 2026-11-24

### Added

- **Tryouts are sold by attempts.** Each product sets how many times a student can take its tryouts per purchase (1 unless the admin changes it). In a bundle every tryout gets that many attempts. Buying the product again adds more attempts.
- **Students see their attempts left.** The tryout page shows how many attempts are used. When none are left, the page and the subtest start page show "Beli kesempatan lagi", which opens that tryout's product.
- **Progress on the tryout result.** Like kecermatan, the result shows the average score of all attempts and the student's rank, next to a chart of the score per attempt. Below the subtest scores, a table lists correct, wrong and blank answers for the last 5 attempts. The score of the attempt being viewed sits under the rank.
- **Subject and chapter filters in the question bank.** A "Filter" button beside the search box opens program, subject, chapter, question type and difficulty together. The same filter works when picking questions for a tryout.
- **Question count in the tryout list,** so admins don't have to open a tryout to see how many questions it has.
- **Plane shapes in questions.** Next to the solids, admins can insert persegi, persegi panjang, segitiga, lingkaran, trapesium, jajar genjang, belah ketupat and layang-layang.

### Changed

- **Picking questions for a tryout.** The picker has the search and filter side by side, a switch between all and selected questions, "Pilih semua" and "Kosongkan", and shows each question's subject and chapter.
- **Result page and PDF are simpler.** The separate correct, total and rank cards are gone; the same numbers are in the average panel and the table. The result PDF follows the page.
- **Editing a question updates tryouts already taken.** The change shows in every tryout that uses the question, and finished attempts are scored again, so scores, averages and ranks follow the corrected answer. The edit form says how many finished attempts will be rescored before saving.
- **Editor toolbar shows icons only,** with the name on hover, so it fits on one line.
- **Wider product form** with fields side by side.
- Buying a tryout you already have is allowed again; it adds attempts instead of being refused.

> **Deploy note:** run migrations `000032_tryout_attempt_credits` and `000033_question_lookup_indexes`. Every existing purchase gets 1 attempt, and students who already took a tryout keep the attempts they used, so they need to buy again for another try.

## v0.1.5 — 2026-11-21

### Added

- **New question editor.** Admins write a question the way it will look: text, formulas, tables, pictures and solid shapes appear in place while typing, with no codes to remember. One toolbar works for the question, every answer option and the explanation.
- **Formulas, tables, pictures and solid shapes in questions.** A formula is built with quick buttons (fraction, power, root and more) and shown before it is inserted. A table is picked by size and filled in directly. A picture is uploaded from the computer (PNG, JPG or WebP, up to 5 MB). Seven solids are ready to insert: kubus, balok, tabung, kerucut, bola, limas and prisma, each with an optional caption.
- **Answer options can hold the same content** as the question, so an option can be a formula, a table or a picture.
- **Question preview.** A "Pratinjau" button shows the question exactly as a student will see it before it is saved.
- **Results per student.** Tryout and kecermatan results list one row per student and test, can be searched by student name, and open a detail page for each student.
- **Result PDF for one student.** The detail page downloads that student's result as a PDF that looks like the page.
- **Result PDF for several students.** On the results pages admins tick up to 30 students and download one PDF with one student per page.

### Changed

- **Add and edit question form.** Writing is on the left (question, answer options, explanation) and the classification and answer settings are on the right.
- **Existing questions keep their content.** Text, formulas and lists written before this version show as before and open in the new editor.
- **Tryout PDF downloads straight away.** "Unduh PDF" in the tryout builder saves the file directly, with a progress count, instead of waiting for an export and an email. Pages are white, every subtest after the first starts on a new page, and there is more room between a subtest's heading and its first question.
- **Students see pictures, tables and solid shapes** in the exam, and the tryout PDF prints them too.
- **Picked avatars show everywhere for staff.** The students, users and monitoring pages, the leaderboard and the admin dashboard show the avatar a student chose.
- **Kecermatan cards are the same size.** The cards of a soal are as wide and tall as the reference cards above them, with the same symbol size.
- The question list shows formulas typeset instead of as code.

### Removed

- **Results export to CSV and XLSX** and the "Ekspor terbaru" list. Results are downloaded as PDFs only.
- The "Ekspor CSV" button on the kecermatan results page.

> **Deploy note:** run migrations `000029_drop_tryout_pdf_export`, `000030_question_rich_docs` and `000031_drop_exports`. The API converts existing questions once at its first start. The Gotenberg container and `GOTENBERG_URL` are no longer used and can be removed. Export files made before this version stay in storage. Text in the PDFs is an image and cannot be selected.

## v0.1.4 — 2026-11-18

### Changed

- **New kecermatan scores.** Students see kecepatan, ketahanan, ketelitian and the final score calculated with Taruna Eduka's revised formula. Ketelitian now counts most towards the final score (40%), with kecepatan and ketahanan at 30% each.
- **Scores no longer reach 100.** A student who answers every soal correctly at a steady pace gets a final score of about 79, so expect lower numbers than before for the same performance.
- **Past results are updated too.** The score history and the progress chart show every earlier attempt with the new formula, so old and new attempts can be compared.
- **Shorter kecermatan tests.** Admins can create a test with as few as 2 kolom, in even numbers (2, 4, 6, up to 50). The minimum used to be 10.

> **Deploy note:** run migration `000028_kecermatan_kolom_from_two` when deploying, then `bootstrap rescore-kecermatan -tenant <slug>` once so stored kecermatan results move to scoring version 2.

## v0.1.3 — 2026-11-10

Run migrations `000026_user_avatar` and `000027_user_gender_birth_date` when deploying.

### Added

- **Kecermatan progress page.** The result page now summarises every attempt of a test: a combined score (average of the last five completed attempts) with a trend against the previous window, a chart of the score per attempt, average kecepatan, ketelitian and ketahanan, and the per-column recap opened per attempt.
- **Avatars.** Every user, staff included, can pick one of 35 built-in avatars from the profile page.
- **Automatic avatar.** A user who has not picked one gets an avatar chosen from their gender and age, or a random one when neither is known.
- **Gender and date of birth.** Students give both at sign-up and can change them on the profile page.
- **Repeat password at sign-up.** The password is typed twice and a mismatch is caught before the form is sent.
- **Staff profile page** at `/admin/profile`, with the avatar and the password change.

### Changed

- **Larger text app-wide.** The small type sizes are 2px larger on every page.
- **More colour on the dashboards.** Student and admin dashboards use tinted metric tiles, programme-tinted rows and avatars, with new blue, green and amber data colours.
- **Password fields are hidden** on the profile page until "Ubah kata sandi" is pressed.
- The kecermatan result page no longer shows the per-attempt review (pembahasan), the `/100` suffix or the benar/salah/kosong tiles.

### API

- `GET /api/v1/my/kecermatan-tests/{id}` returns a `progress` object.
- New `PUT /api/v1/me/avatar`.
- `POST /api/v1/auth/register` requires `gender` and `birth_date`.
- `PATCH /api/v1/me` accepts `gender` and `birth_date`.
- `User` gains `avatar` (never null), `avatar_picked`, `gender` and `birth_date`.

## v0.1.2 — 2026-11-09

### Changed

- Kecermatan attempt layout: aligned columns, labels above the reference row and a status bar at the top.
- Kecermatan items scale to the widest item of a column and shrink before wrapping, so a string is never split.
- Roomier kecermatan header and status bar.

## v0.1.1 — 2026-11-08

### Added

- Visibility toggle for the programme slots shown on the website.

## v0.1.0 — 2026-11-08

First tagged release.
