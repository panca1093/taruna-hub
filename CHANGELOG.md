# Changelog

Notable changes to the Taruna Eduka tryout platform, newest first. Each version is a git tag; pushing a `v*` tag builds the deploy images.

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
