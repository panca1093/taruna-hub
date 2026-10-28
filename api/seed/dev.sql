-- Dev-only fixtures. Deliberately NOT a migration: production must never run this.
-- Mirrors the mockup's currentStudent so web/ shows the same person before and after cutover.
-- Password hash is bcrypt("password", cost 10), verified with golang.org/x/crypto/bcrypt.
-- Two tenants share naila@example.com on purpose: identifiers are unique per tenant (FR-AUTH-07).
-- Dev serves taruna (TENANT_SLUG=taruna); sekolah-b and the platform admin only exist for scope tests.

INSERT INTO tenants (slug, name) VALUES
    ('taruna', 'Taruna Eduka'),
    ('sekolah-b', 'Sekolah B')
ON CONFLICT (slug) DO NOTHING;

-- P05: the curriculum is tenant-owned, so its rows hang off the tenants above.
-- Same programs, subjects and chapters as web/lib/mock-data.ts, in the same order.
INSERT INTO programs (tenant_id, code, name, description, position)
SELECT t.id, v.code, v.name, v.description, v.position
FROM (VALUES
    ('taruna',    'SD',    'Sekolah Dasar',               'Latihan tryout untuk siswa SD kelas 4-6.',  0),
    ('taruna',    'SMP',   'Sekolah Menengah Pertama',    'Latihan tryout untuk siswa SMP kelas 7-9.', 1),
    ('taruna',    'SMA',   'Sekolah Menengah Atas',       'Simulasi UTBK & tryout mapel SMA/SMK.',     2),
    ('taruna',    'CASN',  'Calon Aparatur Sipil Negara', 'Latihan SKD CPNS: TWK, TIU, TKP.',          3),
    ('taruna',    'BUMN',  'Rekrutmen BUMN',              'Persiapan tes rekrutmen bersama BUMN.',     4),
    ('taruna',    'AKPOL', 'Akademi Kepolisian',          'Persiapan seleksi akademik AKPOL.',         5),
    -- SD repeats a code taruna already uses: codes are unique per tenant, not globally.
    ('sekolah-b', 'SMA',   'SMA Sekolah B',               'Program contoh tenant kedua.',              0),
    ('sekolah-b', 'SD',    'SD Sekolah B',                'Program contoh tenant kedua.',              1)
) AS v (slug, code, name, description, position)
JOIN tenants t ON t.slug = v.slug
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO subjects (tenant_id, program_id, name, position)
SELECT t.id, p.id, v.name, v.position
FROM (VALUES
    ('taruna',    'SD',    'Matematika',            0),
    ('taruna',    'SD',    'IPA',                   1),
    ('taruna',    'SD',    'Bahasa Indonesia',      2),
    ('taruna',    'SMP',   'Matematika',            0),
    ('taruna',    'SMP',   'IPA',                   1),
    ('taruna',    'SMP',   'Bahasa Inggris',        2),
    ('taruna',    'SMA',   'Matematika',            0),
    ('taruna',    'SMA',   'Fisika',                1),
    ('taruna',    'SMA',   'Kimia',                 2),
    ('taruna',    'SMA',   'Tes Potensi Skolastik', 3),
    ('taruna',    'CASN',  'TWK',                   0),
    ('taruna',    'CASN',  'TIU',                   1),
    ('taruna',    'CASN',  'TKP',                   2),
    ('taruna',    'BUMN',  'TKD',                   0),
    ('taruna',    'BUMN',  'Wawasan Kebangsaan',    1),
    ('taruna',    'BUMN',  'Bahasa Inggris',        2),
    ('taruna',    'AKPOL', 'PPBN',                  0),
    ('taruna',    'AKPOL', 'TIU',                   1),
    ('sekolah-b', 'SMA',   'Matematika',            0)
) AS v (slug, program_code, name, position)
JOIN tenants t ON t.slug = v.slug
JOIN programs p ON p.tenant_id = t.id AND p.code = v.program_code
ON CONFLICT (program_id, lower(name)) DO NOTHING;

INSERT INTO chapters (tenant_id, subject_id, name, position)
SELECT s.tenant_id, s.id, v.name, v.position
FROM (VALUES
    ('taruna',    'SD',    'Matematika',            'Bilangan',                 0),
    ('taruna',    'SD',    'Matematika',            'Pecahan',                  1),
    ('taruna',    'SD',    'Matematika',            'Bangun Datar',             2),
    ('taruna',    'SD',    'IPA',                   'Makhluk Hidup',            0),
    ('taruna',    'SD',    'IPA',                   'Energi & Perubahannya',    1),
    ('taruna',    'SD',    'Bahasa Indonesia',      'Membaca Pemahaman',        0),
    ('taruna',    'SD',    'Bahasa Indonesia',      'Tata Bahasa',              1),
    ('taruna',    'SMP',   'Matematika',            'Aljabar',                  0),
    ('taruna',    'SMP',   'Matematika',            'Geometri',                 1),
    ('taruna',    'SMP',   'Matematika',            'Bilangan Berpangkat',      2),
    ('taruna',    'SMP',   'IPA',                   'Zat & Perubahannya',       0),
    ('taruna',    'SMP',   'IPA',                   'Klasifikasi Makhluk Hidup', 1),
    ('taruna',    'SMP',   'Bahasa Inggris',        'Tenses',                   0),
    ('taruna',    'SMP',   'Bahasa Inggris',        'Vocabulary',               1),
    ('taruna',    'SMA',   'Matematika',            'Persamaan Kuadrat',        0),
    ('taruna',    'SMA',   'Matematika',            'Kalkulus Dasar',           1),
    ('taruna',    'SMA',   'Matematika',            'Logaritma',                2),
    ('taruna',    'SMA',   'Fisika',                'Mekanika',                 0),
    ('taruna',    'SMA',   'Fisika',                'Listrik & Magnet',         1),
    ('taruna',    'SMA',   'Kimia',                 'Tabel Periodik',           0),
    ('taruna',    'SMA',   'Kimia',                 'Reaksi Kimia',             1),
    ('taruna',    'SMA',   'Tes Potensi Skolastik', 'Penalaran Umum',           0),
    ('taruna',    'SMA',   'Tes Potensi Skolastik', 'Pemahaman Bacaan',         1),
    ('taruna',    'SMA',   'Tes Potensi Skolastik', 'Pengetahuan Kuantitatif',  2),
    ('taruna',    'CASN',  'TWK',                   'Pancasila',                0),
    ('taruna',    'CASN',  'TWK',                   'UUD 1945',                 1),
    ('taruna',    'CASN',  'TWK',                   'Bhinneka Tunggal Ika',     2),
    ('taruna',    'CASN',  'TIU',                   'Analogi & Silogisme',      0),
    ('taruna',    'CASN',  'TIU',                   'Deret Angka',              1),
    ('taruna',    'CASN',  'TIU',                   'Berhitung',                2),
    ('taruna',    'CASN',  'TKP',                   'Pelayanan Publik',         0),
    ('taruna',    'CASN',  'TKP',                   'Kerja Sama Tim',           1),
    ('taruna',    'CASN',  'TKP',                   'Integritas',               2),
    ('taruna',    'BUMN',  'TKD',                   'Verbal',                   0),
    ('taruna',    'BUMN',  'TKD',                   'Numerik',                  1),
    ('taruna',    'BUMN',  'Wawasan Kebangsaan',    'Sejarah Indonesia',        0),
    ('taruna',    'BUMN',  'Wawasan Kebangsaan',    'Lembaga Negara',           1),
    ('taruna',    'BUMN',  'Bahasa Inggris',        'Grammar',                  0),
    ('taruna',    'BUMN',  'Bahasa Inggris',        'Reading Comprehension',    1),
    ('taruna',    'AKPOL', 'PPBN',                  'Tri Brata',                0),
    ('taruna',    'AKPOL', 'PPBN',                  'Catur Prasetya',           1),
    ('taruna',    'AKPOL', 'TIU',                   'Aritmatika',               0),
    ('taruna',    'AKPOL', 'TIU',                   'Deret',                    1),
    ('sekolah-b', 'SMA',   'Matematika',            'Aljabar',                  0),
    ('sekolah-b', 'SMA',   'Matematika',            'Trigonometri',             1)
) AS v (slug, program_code, subject_name, name, position)
JOIN tenants t ON t.slug = v.slug
JOIN programs p ON p.tenant_id = t.id AND p.code = v.program_code
JOIN subjects s ON s.program_id = p.id AND s.name = v.subject_name
ON CONFLICT (subject_id, lower(name)) DO NOTHING;

INSERT INTO users (tenant_id, name, email, phone, password_hash, role, balance, email_verified_at)
SELECT t.id, v.name, v.email, v.phone,
       '$2a$10$gZ0w6wuB5MiODdtWAetlPetM3X84pFE04KlyoP3h0Wsoa43WzhEp.', v.role::user_role, v.balance, now()
FROM (VALUES
    ('taruna',    'Naila Ramadhani',  'naila@example.com',       '+6281234567890', 'student',  150000),
    ('taruna',    'Siti Aminah',      'admin@tarunaeduka.id',    '+6281200000001', 'admin',    0),
    ('taruna',    'Rizky Operator',   'operator@tarunaeduka.id', '+6281200000002', 'operator', 0),
    ('sekolah-b', 'Naila Sekolah B',  'naila@example.com',       '+6281234567890', 'student',  25000),
    ('sekolah-b', 'Admin Sekolah B',  'admin@sekolah-b.test',    '+6281200000003', 'admin',    0)
) AS v (slug, name, email, phone, role, balance)
JOIN tenants t ON t.slug = v.slug
ON CONFLICT DO NOTHING;

INSERT INTO users (tenant_id, name, email, phone, password_hash, role, email_verified_at)
VALUES (NULL, 'Platform Admin', 'root@tarunaeduka.id', '+6281200000009',
        '$2a$10$gZ0w6wuB5MiODdtWAetlPetM3X84pFE04KlyoP3h0Wsoa43WzhEp.', 'platform_admin', now())
ON CONFLICT DO NOTHING;

-- P07a: the question bank. Same 40 questions as web/lib/mock-data.ts for `taruna`, plus two for
-- `sekolah-b` that prove the tenant filter; program, subject and chapter are looked up by name.
-- Ids are derived from the content, so re-running the seed cannot duplicate a question and `seq`
-- stays a plain identity column. The per-difficulty weight of the prototype is dropped (D04 pending).
INSERT INTO questions (id, tenant_id, current_version, created_by)
SELECT v.id::uuid, t.id, 1, u.id
FROM (VALUES
    ('taruna', '0312784b-e8b0-5d44-87c0-2493cfc6f622', 'admin@tarunaeduka.id'),
    ('taruna', '5cc2ebe6-0096-5357-a3f2-9e5862f09bb2', 'admin@tarunaeduka.id'),
    ('taruna', '2dacd664-423a-5693-a847-f5c193c4769d', 'admin@tarunaeduka.id'),
    ('taruna', '56bf865b-c415-5c72-ad03-6ef3c76a95c3', 'admin@tarunaeduka.id'),
    ('taruna', 'e856a268-0195-5dad-a957-6f0932d96556', 'admin@tarunaeduka.id'),
    ('taruna', '77365775-f1e1-5fe3-8f24-dbd0b16a72d4', 'admin@tarunaeduka.id'),
    ('taruna', '6fb7d5e3-f6b0-5bd3-bea2-e5edb2d8d431', 'admin@tarunaeduka.id'),
    ('taruna', '41cec195-d1f6-5d65-88de-1b768e2533ff', 'admin@tarunaeduka.id'),
    ('taruna', '9dd60435-ab25-5275-b745-b2b64f0bd7de', 'admin@tarunaeduka.id'),
    ('taruna', '498129c6-9eba-567a-a417-64c1016a0e64', 'admin@tarunaeduka.id'),
    ('taruna', 'fb898f16-f00b-5a8d-a5b3-a191737a8f59', 'admin@tarunaeduka.id'),
    ('taruna', 'd58280ef-78fc-55df-90f1-d662bd6da7a6', 'admin@tarunaeduka.id'),
    ('taruna', '66af6450-ecd2-5452-91e8-fc5b47851828', 'admin@tarunaeduka.id'),
    ('taruna', 'cb4c9f11-62a9-5594-a4f9-fbb0a505c830', 'admin@tarunaeduka.id'),
    ('taruna', '97f61c55-9471-5625-bbaf-3f94e3b53b39', 'admin@tarunaeduka.id'),
    ('taruna', 'a48065e6-b989-5b12-911a-82105ef27147', 'admin@tarunaeduka.id'),
    ('taruna', '5628bd1b-c7d1-54b3-a8f6-ae1ba8632a57', 'admin@tarunaeduka.id'),
    ('taruna', 'a628c658-3c76-5563-9650-352cc0634df7', 'admin@tarunaeduka.id'),
    ('taruna', '815e41d1-61dc-5695-b2e3-97626f446abc', 'admin@tarunaeduka.id'),
    ('taruna', 'e0a0bd69-e722-5742-b7b9-f3b932b86196', 'admin@tarunaeduka.id'),
    ('taruna', 'd8d0a83f-e082-5a92-beb9-de9e5e2e73ab', 'admin@tarunaeduka.id'),
    ('taruna', '9a6c89e5-2c94-5ec4-b996-611a8b8573a1', 'admin@tarunaeduka.id'),
    ('taruna', '225ec858-8bfc-5b10-ae3d-6ad121e61d08', 'admin@tarunaeduka.id'),
    ('taruna', 'ccc6e954-b718-5440-b666-cabf8808c33e', 'admin@tarunaeduka.id'),
    ('taruna', 'da0d3f93-de4c-5d94-83f6-6b7851e3980c', 'admin@tarunaeduka.id'),
    ('taruna', 'fb916526-e276-502a-b8dc-911fce4897b0', 'admin@tarunaeduka.id'),
    ('taruna', 'b1a7b7a1-b396-5549-8fe7-dccafe06df58', 'admin@tarunaeduka.id'),
    ('taruna', '05656c13-a388-50cb-be05-fc39631f2506', 'admin@tarunaeduka.id'),
    ('taruna', '635bf86e-d0c4-5ba4-9c45-f72f2eeef3ba', 'admin@tarunaeduka.id'),
    ('taruna', 'db446b42-09de-59c9-9675-9809b46ac6de', 'admin@tarunaeduka.id'),
    ('taruna', '28229342-46e9-5e32-95d8-d8647b86210e', 'admin@tarunaeduka.id'),
    ('taruna', 'bb3982e7-2dfa-5498-a79d-f01b8a649b1e', 'admin@tarunaeduka.id'),
    ('taruna', '4a50d964-34aa-5b35-b199-61c6ca5ade68', 'admin@tarunaeduka.id'),
    ('taruna', 'bb69037a-b7dc-5d3c-8f99-ce00939ff340', 'admin@tarunaeduka.id'),
    ('taruna', 'd4f897a4-8117-5f11-9546-8a5fdb679f9d', 'admin@tarunaeduka.id'),
    ('taruna', 'b64ba100-6da2-520d-a083-d8dfa04faf65', 'admin@tarunaeduka.id'),
    ('taruna', 'f63ea9b5-0185-58c6-8fe7-3988a1e9f406', 'admin@tarunaeduka.id'),
    ('taruna', 'aa60dfb1-6555-504c-834d-91f67d302a75', 'admin@tarunaeduka.id'),
    ('taruna', 'b324bda9-37c9-5758-8b1b-8cf733f40774', 'admin@tarunaeduka.id'),
    ('taruna', 'e0e0ec21-78d9-54fa-895b-92f4fcca0369', 'admin@tarunaeduka.id'),
    ('sekolah-b', 'b6f09a2c-0c54-54f5-912d-be121802d495', 'admin@sekolah-b.test'),
    ('sekolah-b', '364a5b78-750b-596b-878c-7076cf6caf09', 'admin@sekolah-b.test')
) AS v (slug, id, author_email)
JOIN tenants t ON t.slug = v.slug
JOIN users u ON u.tenant_id = t.id AND u.email = v.author_email
ON CONFLICT (id) DO NOTHING;

INSERT INTO question_versions (id, question_id, tenant_id, version, program_id, subject_id, chapter_id,
        type, content, difficulty, explanation, option_weighted, case_sensitive, options, correct, created_by)
SELECT v.version_id::uuid, v.question_id::uuid, t.id, 1, p.id, s.id, c.id,
       v.type, v.content, v.difficulty, v.explanation, v.weighted::boolean, false,
       v.options::jsonb, v.correct::jsonb, u.id
FROM (VALUES
('taruna', '0312784b-e8b0-5d44-87c0-2493cfc6f622', '6fae898d-6074-574d-adcf-e12f3ddaad4d', 'SD', 'Matematika', NULL, 'pg', 'Hasil dari 248 + 179 adalah ...', 'easy', '248 + 179 = 427.', false, '[{"id": "80e745c0-05d1-5f49-8495-b697e27f57cc", "label": "417"}, {"id": "0e656d4a-1a5e-5cb1-a40a-42c7627f01d8", "label": "427"}, {"id": "322f8cc3-dc91-50be-bd85-0f8ca30c6a80", "label": "437"}, {"id": "df2aa70a-e74a-57e1-aa9a-21a97bfac32e", "label": "447"}]'::jsonb, '["0e656d4a-1a5e-5cb1-a40a-42c7627f01d8"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '5cc2ebe6-0096-5357-a3f2-9e5862f09bb2', '789dc478-bb7a-5a75-9f4c-ca7b2ba4501a', 'SD', 'Matematika', NULL, 'pg', '7 × 8 = ...', 'easy', 'Perkalian dasar: 7 × 8 = 56.', false, '[{"id": "2dc2b071-2705-5ed5-a784-bbb3ce0e146e", "label": "54"}, {"id": "6a6aa373-53b2-5961-b9ea-d30bbbb1a95c", "label": "56"}, {"id": "edcd5a1f-7a08-5cb7-bdea-d87e65bf2ae6", "label": "58"}, {"id": "3b86aaf9-c0d9-5808-85c9-6ee03d490b0a", "label": "64"}]'::jsonb, '["6a6aa373-53b2-5961-b9ea-d30bbbb1a95c"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '2dacd664-423a-5693-a847-f5c193c4769d', 'a274ffb8-dc50-552c-b4fe-db4cd4a72952', 'SD', 'Matematika', NULL, 'isian', 'Keliling persegi dengan panjang sisi 9 cm adalah ... cm', 'medium', 'Keliling persegi = 4 × sisi = 4 × 9 = 36 cm.', false, '[]'::jsonb, '["36"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '56bf865b-c415-5c72-ad03-6ef3c76a95c3', '6d755b8e-7624-581f-8dcb-2f3de19d76d9', 'SD', 'IPA', NULL, 'benar_salah', 'Matahari terbit dari arah timur.', 'easy', 'Pernyataan ini benar — rotasi bumi menyebabkan matahari tampak terbit dari timur.', false, '[{"id": "benar", "label": "Benar"}, {"id": "salah", "label": "Salah"}]'::jsonb, '["benar"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'e856a268-0195-5dad-a957-6f0932d96556', 'c319760d-7d04-517e-898a-98a68a858ba0', 'SD', 'IPA', NULL, 'pg_multi', 'Berikut ini yang termasuk sumber energi terbarukan adalah ...', 'medium', 'Matahari, angin, dan air adalah sumber energi terbarukan karena dapat diperbarui secara alami.', false, '[{"id": "f4a397fa-6d9d-554c-b190-492d54c1c70f", "label": "Matahari"}, {"id": "f04aaea1-7c1b-5a3f-8f65-bade274df3d4", "label": "Angin"}, {"id": "e54794d3-5f41-593a-82a2-f06b3b2185b6", "label": "Air"}, {"id": "90302674-8bcd-5b59-a2ea-7ad2191e821a", "label": "Batu bara"}]'::jsonb, '["f4a397fa-6d9d-554c-b190-492d54c1c70f", "f04aaea1-7c1b-5a3f-8f65-bade274df3d4", "e54794d3-5f41-593a-82a2-f06b3b2185b6"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '77365775-f1e1-5fe3-8f24-dbd0b16a72d4', '08282b23-ed7b-5fa6-b718-1f484d67a972', 'SD', 'Bahasa Indonesia', NULL, 'pg', 'Sinonim dari kata ''cerdas'' adalah ...', 'easy', 'Sinonim (persamaan makna) dari ''cerdas'' adalah ''pintar''.', false, '[{"id": "5de4f307-2183-5697-9c7e-1f43bd1f5dd5", "label": "Bodoh"}, {"id": "83151767-788c-58f7-b6dc-fd2dcddda8bf", "label": "Pintar"}, {"id": "04b9fd73-a00b-5afc-8463-f3b694c3ac10", "label": "Malas"}, {"id": "30d773d6-1d69-5bd8-9675-dd392794d0e9", "label": "Lambat"}]'::jsonb, '["83151767-788c-58f7-b6dc-fd2dcddda8bf"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '6fb7d5e3-f6b0-5bd3-bea2-e5edb2d8d431', '5bb966ec-cff8-516e-9a40-592095ef440d', 'SD', 'Matematika', NULL, 'isian', 'Bilangan yang tepat setelah 999 adalah ...', 'easy', 'Urutan bilangan asli setelah 999 adalah 1000.', false, '[]'::jsonb, '["1000"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '41cec195-d1f6-5d65-88de-1b768e2533ff', 'b88eff5b-64de-54c0-83ff-eb1aaeffc5ad', 'SD', 'Matematika', NULL, 'pg', 'Ibu membeli 3 lusin telur. Jumlah telur yang dibeli ibu adalah ...', 'medium', '1 lusin = 12 butir, sehingga 3 lusin = 3 × 12 = 36 butir.', false, '[{"id": "924591f3-f897-59ab-a40b-9ae97f464bb5", "label": "24"}, {"id": "4d5901b2-e0cb-5613-9f38-b1afe43745d6", "label": "30"}, {"id": "7dc813e0-d60a-5f34-8198-73b2da61270f", "label": "36"}, {"id": "6dfd575b-029f-520d-9022-d6943f331df6", "label": "42"}]'::jsonb, '["7dc813e0-d60a-5f34-8198-73b2da61270f"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '9dd60435-ab25-5275-b745-b2b64f0bd7de', '93708734-b6dd-5ca0-ab27-41c185d510fc', 'SMP', 'Matematika', NULL, 'pg', 'Jika $2x + 5 = 17$, maka nilai $x$ adalah ...', 'medium', '$2x = 17 - 5 = 12$, sehingga $x = 6$.', false, '[{"id": "734dbd4e-bf55-5557-9ba5-9d0bee658fa4", "label": "5"}, {"id": "cbe7d66b-73d1-527e-9490-58f73714ebc2", "label": "6"}, {"id": "49ac3ae2-e7af-5940-8a42-a61435ae4c24", "label": "7"}, {"id": "c6a5b32c-df3b-5911-81ae-4dbd71ee09de", "label": "8"}]'::jsonb, '["cbe7d66b-73d1-527e-9490-58f73714ebc2"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '498129c6-9eba-567a-a417-64c1016a0e64', '6e24e916-d11c-5c1c-bbdd-5f2a56944898', 'SMP', 'IPA', NULL, 'pg_multi', 'Manakah di antara berikut yang termasuk perubahan kimia?', 'hard', 'Kertas dibakar dan besi berkarat menghasilkan zat baru (perubahan kimia). Es mencair dan lilin meleleh adalah perubahan fisika.', false, '[{"id": "157bdcbe-3902-511f-8917-1978e74863cd", "label": "Kertas dibakar"}, {"id": "9a43cadb-451d-53bd-aabc-dbe441d8b2df", "label": "Es mencair"}, {"id": "3f140df6-a1d0-5486-af02-e2c30a6ca669", "label": "Besi berkarat"}, {"id": "258e61fa-c3a1-5955-8794-4682e1d35c6d", "label": "Lilin meleleh"}]'::jsonb, '["157bdcbe-3902-511f-8917-1978e74863cd", "3f140df6-a1d0-5486-af02-e2c30a6ca669"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'fb898f16-f00b-5a8d-a5b3-a191737a8f59', '6df5f9de-a92b-5f82-b85d-90832251920e', 'SMP', 'IPA', NULL, 'benar_salah', 'Fotosintesis pada tumbuhan menghasilkan oksigen.', 'medium', 'Benar — fotosintesis mengubah CO2 dan air menjadi glukosa dan oksigen.', false, '[{"id": "benar", "label": "Benar"}, {"id": "salah", "label": "Salah"}]'::jsonb, '["benar"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'd58280ef-78fc-55df-90f1-d662bd6da7a6', '250b3eca-7f5f-5ac0-90df-e63c95a79ad5', 'SMP', 'Matematika', NULL, 'isian', 'Nilai dari $\sqrt{144}$ adalah ...', 'medium', '$\sqrt{144} = 12$ karena $12 \times 12 = 144$.', false, '[]'::jsonb, '["12"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '66af6450-ecd2-5452-91e8-fc5b47851828', '7b5366e2-a69a-5d62-8a1a-d33172c2789b', 'SMP', 'Bahasa Inggris', NULL, 'pg', 'The past tense of ''go'' is ...', 'easy', 'Bentuk lampau (past tense) dari ''go'' adalah ''went''.', false, '[{"id": "d9a79c51-ba5f-5164-ba5e-b1afde53232c", "label": "goes"}, {"id": "afe7378c-d916-5bc1-adc3-0315b43f19ff", "label": "gone"}, {"id": "08168360-83fd-5880-b4b6-3921234c255f", "label": "went"}, {"id": "374db21e-82ef-5860-954f-e05a8207253d", "label": "going"}]'::jsonb, '["08168360-83fd-5880-b4b6-3921234c255f"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'cb4c9f11-62a9-5594-a4f9-fbb0a505c830', '8520a41e-536c-59f2-8c00-ce32d319f2b6', 'SMP', 'Matematika', NULL, 'pg', 'Luas permukaan kubus dengan panjang rusuk 6 cm adalah ... $cm^2$', 'hard', 'Luas permukaan kubus = $6 \times s^2 = 6 \times 36 = 216\ cm^2$.', false, '[{"id": "084eb5c6-cc20-562d-a29a-ff41f6d79130", "label": "196"}, {"id": "2083ef0a-c9ee-5922-a09b-0f1e054b3a76", "label": "206"}, {"id": "9af90e2e-e3d2-5675-a030-0be711d01d2e", "label": "216"}, {"id": "e1294c6b-f9c7-57b0-bd77-982ee376bfa8", "label": "226"}]'::jsonb, '["9af90e2e-e3d2-5675-a030-0be711d01d2e"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '97f61c55-9471-5625-bbaf-3f94e3b53b39', '5767c034-d981-58a8-8ebe-d4c34454331b', 'SMP', 'Bahasa Inggris', NULL, 'pg_multi', 'Pilih bentuk jamak (plural) yang tepat:', 'medium', 'Bentuk jamak yang benar adalah ''children'' (dari child) dan ''mice'' (dari mouse).', false, '[{"id": "ff0164ac-e955-5c84-9baa-1e44b751f6cd", "label": "childs"}, {"id": "26495794-cecd-5152-985c-48023f0af698", "label": "children"}, {"id": "dc9f5b73-b2e6-51fb-a615-e3f50fe61d09", "label": "mice"}, {"id": "3854ce21-f2ff-5a2c-9f83-08d7a299e3cb", "label": "mouses"}]'::jsonb, '["26495794-cecd-5152-985c-48023f0af698", "dc9f5b73-b2e6-51fb-a615-e3f50fe61d09"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'a48065e6-b989-5b12-911a-82105ef27147', '5158c57f-875b-512d-acc0-5390ae546520', 'SMA', 'Matematika', NULL, 'pg', 'Akar-akar persamaan $x^2 - 5x + 6 = 0$ adalah ...', 'hard', 'Faktorkan: $(x-2)(x-3)=0$, sehingga akar-akarnya $x = 2$ dan $x = 3$.', false, '[{"id": "8c47d37b-9c58-532c-a0d9-34eed1e1b9ba", "label": "2 dan 3"}, {"id": "1f5292dd-16f6-5b63-906d-d0eb019a3921", "label": "1 dan 6"}, {"id": "8e32c98f-1e5f-560a-8a8d-55f4d99e2f4e", "label": "-2 dan -3"}, {"id": "89ed23dc-27fb-5a19-a7ad-592666650610", "label": "2 dan -3"}]'::jsonb, '["8c47d37b-9c58-532c-a0d9-34eed1e1b9ba"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '5628bd1b-c7d1-54b3-a8f6-ae1ba8632a57', '7221bdb2-4df6-5a10-9ad1-dc1021e349fa', 'SMA', 'Matematika', NULL, 'isian', 'Nilai dari $\log_2 32$ adalah ...', 'hard', '$\log_2 32 = 5$ karena $2^5 = 32$.', false, '[]'::jsonb, '["5"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'a628c658-3c76-5563-9650-352cc0634df7', 'a8760197-addb-5fdf-88ff-9b7d7b91137a', 'SMA', 'Fisika', NULL, 'pg', 'Satuan SI untuk gaya adalah ...', 'medium', 'Satuan gaya dalam Sistem Internasional (SI) adalah newton (N).', false, '[{"id": "56a56854-0c30-5743-a732-fbbca9d729ef", "label": "Joule"}, {"id": "d8681fea-65a6-59c3-805b-850e1d6fa7ae", "label": "Newton"}, {"id": "6183df19-a0ab-5ad9-b7b8-43879c6189d9", "label": "Watt"}, {"id": "58d1db06-f42c-571b-9eaa-5dc692755c0d", "label": "Pascal"}]'::jsonb, '["d8681fea-65a6-59c3-805b-850e1d6fa7ae"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '815e41d1-61dc-5695-b2e3-97626f446abc', 'efcf2a88-2c69-5591-9e40-8af39d2b40aa', 'SMA', 'Kimia', NULL, 'pg_multi', 'Manakah unsur berikut yang tergolong logam alkali?', 'hard', 'Logam alkali berada pada golongan IA: Natrium (Na), Kalium (K), dan Litium (Li). Kalsium tergolong logam alkali tanah.', false, '[{"id": "0a6ad92d-256d-56bd-a871-90f58ab88b85", "label": "Natrium"}, {"id": "03cfb9ea-cea6-50da-8ae2-59c32302b477", "label": "Kalium"}, {"id": "45d179b4-a612-5d47-9617-6dd182e1fcf2", "label": "Kalsium"}, {"id": "ec43aca7-9c7e-5802-879d-e3ceb2e04938", "label": "Litium"}]'::jsonb, '["0a6ad92d-256d-56bd-a871-90f58ab88b85", "03cfb9ea-cea6-50da-8ae2-59c32302b477", "ec43aca7-9c7e-5802-879d-e3ceb2e04938"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'e0a0bd69-e722-5742-b7b9-f3b932b86196', '9b2fcb67-1d22-52d8-8a20-f40eb8bf59ff', 'SMA', 'Fisika', NULL, 'benar_salah', 'Percepatan gravitasi rata-rata di permukaan bumi adalah 9,8 m/s².', 'medium', 'Benar — nilai ini adalah nilai standar percepatan gravitasi bumi yang umum digunakan.', false, '[{"id": "benar", "label": "Benar"}, {"id": "salah", "label": "Salah"}]'::jsonb, '["benar"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'd8d0a83f-e082-5a92-beb9-de9e5e2e73ab', 'a4ab41cc-6b1f-59a7-aab0-0f2b28c11e30', 'SMA', 'Matematika', NULL, 'pg', 'Turunan pertama dari $f(x) = 3x^2 + 2x$ adalah ...', 'hard', 'Menggunakan aturan pangkat: $f''(x) = 6x + 2$.', false, '[{"id": "5a90ff7e-4393-59b8-aeed-67f1d1fdd551", "label": "6x + 2"}, {"id": "c809c7a4-fe6a-5b43-a7db-6113771d750e", "label": "3x + 2"}, {"id": "84188747-2f48-554b-b33f-7336793d0e8b", "label": "6x"}, {"id": "0a505a46-e0c6-5b82-8d3e-7d9e04b1be2c", "label": "x^2"}]'::jsonb, '["5a90ff7e-4393-59b8-aeed-67f1d1fdd551"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '9a6c89e5-2c94-5ec4-b996-611a8b8573a1', '4c6f8ea3-a928-55d8-b401-8f1b346c9f1a', 'SMA', 'Tes Potensi Skolastik', NULL, 'pg', 'Semua siswa yang rajin belajar lulus ujian. Andi lulus ujian. Maka ...', 'medium', 'Ini adalah kekeliruan logika ''affirming the consequent'' — lulus ujian tidak otomatis berarti rajin belajar, bisa ada sebab lain.', false, '[{"id": "ed27e6e7-5698-5a09-a8a0-82712c5af384", "label": "Andi pasti rajin belajar"}, {"id": "75f79f64-cd85-5ff5-88d1-bb816c2d9e9d", "label": "Andi belum tentu rajin belajar"}, {"id": "ddf55dfd-a0ac-5d8f-8ba5-f0527733eaf1", "label": "Andi tidak rajin belajar"}, {"id": "e53c5112-56b1-5fb6-bbe0-dfe17202362f", "label": "Tidak dapat disimpulkan sama sekali"}]'::jsonb, '["75f79f64-cd85-5ff5-88d1-bb816c2d9e9d"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '225ec858-8bfc-5b10-ae3d-6ad121e61d08', 'a01e841d-08ae-5b8b-8af5-3025106c072b', 'SMA', 'Kimia', NULL, 'pg', 'Rumus kimia untuk garam dapur adalah ...', 'easy', 'Garam dapur (natrium klorida) memiliki rumus kimia NaCl.', false, '[{"id": "a1dbf8df-5227-58c2-b0ff-f0919e66d1ed", "label": "NaCl"}, {"id": "ea6c6a0e-2bc3-5611-a1fe-ad04bd22086e", "label": "KCl"}, {"id": "aad42af7-24cf-5f03-9009-e99214e320d2", "label": "CaCl2"}, {"id": "4f41d10a-7133-516a-869f-b77d7fe29245", "label": "NaOH"}]'::jsonb, '["a1dbf8df-5227-58c2-b0ff-f0919e66d1ed"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'ccc6e954-b718-5440-b666-cabf8808c33e', '95afe385-a503-5af8-90b0-3fce3e4156e1', 'SMA', 'Fisika', NULL, 'pg', 'Sebuah benda bermassa 2 kg bergerak dengan percepatan 3 m/s². Gaya yang bekerja pada benda adalah ... N', 'medium', 'Menggunakan hukum Newton II: $F = m \times a = 2 \times 3 = 6$ N.', false, '[{"id": "f85fa929-fc6e-5b65-888c-c98de79774d3", "label": "5"}, {"id": "c1f777b9-0ed3-5172-902d-6fc33e4b2a7c", "label": "6"}, {"id": "d1855f11-3882-50aa-96d3-e4d68a8f3b80", "label": "8"}, {"id": "ccecb005-54ee-5da9-97e9-2a8d0f06d433", "label": "10"}]'::jsonb, '["c1f777b9-0ed3-5172-902d-6fc33e4b2a7c"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'da0d3f93-de4c-5d94-83f6-6b7851e3980c', '61701999-4741-5276-8c8e-59e179c9ba74', 'SMA', 'Tes Potensi Skolastik', NULL, 'pg', 'Kata ''OPTIMIS'' berlawanan makna (antonim) dengan ...', 'medium', 'Antonim dari ''optimis'' (penuh harap) adalah ''pesimis'' (mudah putus asa).', false, '[{"id": "3fe843b0-fd8a-5697-9ff5-ea9eda8a88c9", "label": "Percaya diri"}, {"id": "1f6e9b64-0100-550f-b5e3-d8a27e7d2695", "label": "Yakin"}, {"id": "266d7677-be5e-5351-ad96-5aa1d0a2ff16", "label": "Pesimis"}, {"id": "88b552b5-12d8-5685-a636-501eb746b882", "label": "Ceria"}]'::jsonb, '["266d7677-be5e-5351-ad96-5aa1d0a2ff16"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'fb916526-e276-502a-b8dc-911fce4897b0', '5904daf8-c490-51be-92be-ea913127f7a7', 'SMA', 'Tes Potensi Skolastik', NULL, 'isian', '2, 6, 12, 20, 30, ... Angka selanjutnya dalam deret tersebut adalah ...', 'medium', 'Pola selisih bertambah 2 setiap suku: +4, +6, +8, +10, +12. Maka suku berikutnya 30 + 12 = 42.', false, '[]'::jsonb, '["42"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'b1a7b7a1-b396-5549-8fe7-dccafe06df58', 'cdd2177a-b3b4-5443-8426-0f7c2f988d30', 'CASN', 'TWK', NULL, 'benar_salah', 'Pancasila disahkan sebagai dasar negara Republik Indonesia pada 18 Agustus 1945.', 'medium', 'Benar — Pancasila disahkan pada sidang PPKI tanggal 18 Agustus 1945 bersamaan dengan UUD 1945.', false, '[{"id": "benar", "label": "Benar"}, {"id": "salah", "label": "Salah"}]'::jsonb, '["benar"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '05656c13-a388-50cb-be05-fc39631f2506', '59394a1c-29e8-53a0-912b-175cb2900f22', 'CASN', 'TWK', NULL, 'pg', 'Lambang negara Indonesia adalah ...', 'medium', 'Lambang negara Indonesia adalah Garuda Pancasila.', false, '[{"id": "8a8c946d-7ff5-514b-995b-5d056aef2442", "label": "Garuda Pancasila"}, {"id": "eb9bca9e-b07b-5f31-8cc4-1ac6374a950b", "label": "Bhinneka Tunggal Ika"}, {"id": "bc5e10ff-abec-5f9e-be5d-bd4d6a1bc917", "label": "Merah Putih"}, {"id": "4de1d6fd-21cf-5154-8125-efab8cdbb379", "label": "Pancasila"}]'::jsonb, '["8a8c946d-7ff5-514b-995b-5d056aef2442"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '635bf86e-d0c4-5ba4-9c45-f72f2eeef3ba', 'c46963d7-9e5b-5277-8770-23145741b863', 'CASN', 'TIU', NULL, 'pg', 'Jika semua A adalah B, dan sebagian B adalah C, maka ...', 'hard', 'Karena hanya sebagian B yang C, tidak semua A pasti C — hanya kemungkinan sebagian A adalah C.', false, '[{"id": "f5d4207c-483d-5fff-9ff8-ccdbbaaa63a2", "label": "Semua A adalah C"}, {"id": "0e7f770b-f501-51c9-9bb1-a980859990e8", "label": "Sebagian A mungkin C"}, {"id": "4e3b4409-1409-5895-9e09-46bef90e6312", "label": "Semua C adalah A"}, {"id": "0f82af0e-d9e8-557e-b399-b8afdfde6442", "label": "Tidak ada A yang C"}]'::jsonb, '["0e7f770b-f501-51c9-9bb1-a980859990e8"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'db446b42-09de-59c9-9675-9809b46ac6de', '3c42329e-cfb3-595a-94f8-f77019b74dd6', 'CASN', 'TIU', NULL, 'isian', '8, 11, 15, 20, 26, ... Angka selanjutnya dalam deret tersebut adalah ...', 'medium', 'Pola selisih bertambah 1 setiap suku: +3, +4, +5, +6, +7. Maka suku berikutnya 26 + 7 = 33.', false, '[]'::jsonb, '["33"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '28229342-46e9-5e32-95d8-d8647b86210e', 'b3068535-27d3-5e2d-8588-1d4670a893c1', 'CASN', 'TKP', 'Integritas', 'pg', 'Rekan kerja Anda melakukan kesalahan fatal dalam laporan. Sikap paling tepat adalah ...', 'easy', 'Soal TKP tidak memiliki jawaban salah mutlak — tiap opsi diberi bobot sesuai seberapa ideal sikap tersebut. Melaporkan secara objektif ke atasan (bobot tertinggi) adalah sikap paling profesional.', true, '[{"id": "23a2eb7c-6472-5b3e-b84c-32838d33a288", "label": "Membiarkan saja"}, {"id": "668be9e0-38f3-5145-bf88-d2ea382ad689", "label": "Melaporkan ke atasan secara objektif", "points": 4}, {"id": "b2c43a91-2e60-5e12-b3ed-7f5f8d4efea3", "label": "Menyalahkan di depan umum", "points": 1}, {"id": "c2114a4b-0647-5771-b709-6318b5ca92f4", "label": "Memperbaiki sendiri tanpa melapor", "points": 3}]'::jsonb, '[]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'bb3982e7-2dfa-5498-a79d-f01b8a649b1e', 'eef239bb-ce43-546f-93c3-021c26a45ea3', 'CASN', 'TWK', NULL, 'pg_multi', 'Berikut ini termasuk hak asasi manusia yang diatur dalam UUD 1945 adalah ...', 'medium', 'UUD 1945 mengatur hak hidup, hak mendapat pendidikan, dan hak beragama sebagai hak asasi manusia. Memaksakan kehendak bukan hak, melainkan pelanggaran.', false, '[{"id": "56918387-6782-5df0-9498-30afeba4a119", "label": "Hak hidup"}, {"id": "e6c2279d-cb9c-5c37-b660-3b9d7ebf0b0a", "label": "Hak mendapat pendidikan"}, {"id": "1bd8bcde-355b-5d88-a841-f54ce4954263", "label": "Hak memaksakan kehendak"}, {"id": "cc3022da-2c9d-5521-adbb-b450be6b3841", "label": "Hak beragama"}]'::jsonb, '["56918387-6782-5df0-9498-30afeba4a119", "e6c2279d-cb9c-5c37-b660-3b9d7ebf0b0a", "cc3022da-2c9d-5521-adbb-b450be6b3841"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', '4a50d964-34aa-5b35-b199-61c6ca5ade68', '95022f93-f1ee-5bc5-a78c-00f7506e09cf', 'BUMN', 'TKD', NULL, 'pg', 'BUMN merupakan singkatan dari ...', 'medium', 'BUMN adalah singkatan dari Badan Usaha Milik Negara.', false, '[{"id": "9139f008-dd2f-50c0-8afc-ed904363ce0b", "label": "Badan Usaha Milik Negara"}, {"id": "d373aaef-e711-515d-9352-5c9c57fa4d40", "label": "Badan Usaha Milik Nasional"}, {"id": "d8033df0-daf0-5169-a0f9-9dd4a49efb1b", "label": "Badan Umum Milik Negara"}, {"id": "5ad686f7-fc09-5bc7-82b6-3894b6c02ac3", "label": "Badan Usaha Modal Negara"}]'::jsonb, '["9139f008-dd2f-50c0-8afc-ed904363ce0b"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'bb69037a-b7dc-5d3c-8f99-ce00939ff340', '6eb8ddf3-356f-5eb2-bfbe-8ad3b26e5d07', 'BUMN', 'TKD', NULL, 'isian', 'Jika harga barang Rp150.000 didiskon 20%, harga setelah diskon adalah Rp ...', 'easy', 'Diskon 20% dari 150.000 = 30.000, sehingga harga akhir = 150.000 - 30.000 = 120.000.', false, '[]'::jsonb, '["120000"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'd4f897a4-8117-5f11-9546-8a5fdb679f9d', '30cd7168-d8e0-5f22-b30d-66ffb1d2f676', 'BUMN', 'Wawasan Kebangsaan', NULL, 'benar_salah', 'OJK bertugas mengawasi dan mengatur sektor jasa keuangan di Indonesia.', 'medium', 'Benar — Otoritas Jasa Keuangan (OJK) adalah lembaga independen yang mengawasi sektor jasa keuangan.', false, '[{"id": "benar", "label": "Benar"}, {"id": "salah", "label": "Salah"}]'::jsonb, '["benar"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'b64ba100-6da2-520d-a083-d8dfa04faf65', 'b8676bd8-fd10-5c80-84f8-010bc932f9e6', 'BUMN', 'Bahasa Inggris', NULL, 'pg', 'Choose the correct sentence:', 'medium', 'Untuk subjek tunggal (she), kata kerja bantu negatif yang benar adalah ''doesn''t'' diikuti kata kerja dasar ''like''.', false, '[{"id": "55ec4c4a-d52e-5df5-a2ce-6515af2aa16d", "label": "She don''t like coffee"}, {"id": "170c2379-3180-54ad-b8f8-358d8ea179aa", "label": "She doesn''t likes coffee"}, {"id": "a50f4ff3-e2aa-5120-847a-c55e16de4d0c", "label": "She doesn''t like coffee"}, {"id": "b4cb626a-03a0-5840-96f6-570484c9151d", "label": "She not like coffee"}]'::jsonb, '["a50f4ff3-e2aa-5120-847a-c55e16de4d0c"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'f63ea9b5-0185-58c6-8fe7-3988a1e9f406', '29cad83a-9834-5ab1-b11c-b1fec07640a3', 'AKPOL', 'PPBN', NULL, 'pg', 'Tri Brata merupakan pedoman hidup bagi anggota ...', 'medium', 'Tri Brata adalah pedoman hidup dan kode etik bagi anggota Kepolisian Republik Indonesia (Polri).', false, '[{"id": "f348f638-7e46-5ee2-95cd-f9d92cb23d05", "label": "TNI"}, {"id": "05ca1197-0123-5ac3-ad8f-2aec373c5d29", "label": "Polri"}, {"id": "b30eada3-a01e-504a-a96a-af4946adcde0", "label": "ASN"}, {"id": "1a04a5d5-66e7-572e-8447-94f59783c97c", "label": "Mahasiswa"}]'::jsonb, '["05ca1197-0123-5ac3-ad8f-2aec373c5d29"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'aa60dfb1-6555-504c-834d-91f67d302a75', '38ab2e54-9d8b-5a10-8456-2f2f642d047f', 'AKPOL', 'PPBN', NULL, 'benar_salah', 'Catur Prasetya adalah pedoman kerja bagi anggota Polri.', 'medium', 'Benar — Catur Prasetya berisi 4 komitmen kerja anggota Polri dalam mengabdi kepada masyarakat.', false, '[{"id": "benar", "label": "Benar"}, {"id": "salah", "label": "Salah"}]'::jsonb, '["benar"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'b324bda9-37c9-5758-8b1b-8cf733f40774', '600248bc-8d2f-5863-9ab0-e0bb6833f967', 'AKPOL', 'TIU', NULL, 'pg', 'Sebuah mobil menempuh jarak 240 km dalam waktu 3 jam. Kecepatan rata-rata mobil tersebut adalah ... km/jam', 'hard', 'Kecepatan = jarak ÷ waktu = 240 ÷ 3 = 80 km/jam.', false, '[{"id": "d44a49d0-a694-54ca-91e0-cf0ce096aa93", "label": "60"}, {"id": "734f894f-30df-55ea-be38-bb9f75ad51c5", "label": "70"}, {"id": "0eeac2b9-371c-57f0-a76b-1629ad49b21a", "label": "80"}, {"id": "1a81094c-bb41-5fff-8f98-676cb7ebe53d", "label": "90"}]'::jsonb, '["0eeac2b9-371c-57f0-a76b-1629ad49b21a"]'::jsonb, 'admin@tarunaeduka.id'),
('taruna', 'e0e0ec21-78d9-54fa-895b-92f4fcca0369', '541fe5d7-7b6c-5739-85da-4da4d6b265e8', 'AKPOL', 'TIU', NULL, 'isian', '5, 10, 20, 40, 80, ... Angka selanjutnya dalam deret tersebut adalah ...', 'medium', 'Pola deret geometri dengan rasio 2 (dikali 2 setiap suku). Maka suku berikutnya 80 × 2 = 160.', false, '[]'::jsonb, '["160"]'::jsonb, 'admin@tarunaeduka.id'),
('sekolah-b', 'b6f09a2c-0c54-54f5-912d-be121802d495', '919501fb-69d9-5364-b29b-7ca19ff20009', 'SMA', 'Matematika', 'Aljabar', 'pg', 'Hasil dari $3x + 2x$ adalah ...', 'easy', 'Suku sejenis dijumlahkan: $3x + 2x = 5x$.', false, '[{"id": "e3f2b3c9-64ee-5e7f-9f97-46fceff045c0", "label": "5x"}, {"id": "b93f39bf-f85e-5d29-aa19-b3ef97b4dbfb", "label": "6x"}]'::jsonb, '["e3f2b3c9-64ee-5e7f-9f97-46fceff045c0"]'::jsonb, 'admin@sekolah-b.test'),
('sekolah-b', '364a5b78-750b-596b-878c-7076cf6caf09', 'ef726770-3bcd-5d4b-881b-65b1bb496321', 'SMA', 'Matematika', 'Trigonometri', 'isian', 'Nilai dari $\sin 30^\circ$ adalah ...', 'medium', '$\sin 30^\circ = 0{,}5$.', false, '[]'::jsonb, '["0,5", "1/2"]'::jsonb, 'admin@sekolah-b.test')
) AS v (slug, question_id, version_id, program_code, subject_name, chapter_name, type, content,
        difficulty, explanation, weighted, options, correct, author_email)
JOIN tenants t ON t.slug = v.slug
JOIN programs p ON p.tenant_id = t.id AND p.code = v.program_code
JOIN subjects s ON s.program_id = p.id AND s.name = v.subject_name
LEFT JOIN chapters c ON c.subject_id = s.id AND c.name = v.chapter_name
JOIN users u ON u.tenant_id = t.id AND u.email = v.author_email
ON CONFLICT (question_id, version) DO NOTHING;

-- P08: tryouts over the seeded questions. Rows are looked up by tenant, title and content,
-- and every insert is guarded, so re-running the seed changes nothing.
INSERT INTO tryouts (tenant_id, program_id, title, description, scoring_mode, status,
        active_from, active_until, explanation_release_at, randomize_questions, randomize_options,
        first_attempt_at, created_by)
SELECT t.id, p.id, v.title, v.description, v.scoring_mode, v.status,
       CASE WHEN v.active_offset IS NULL THEN NULL ELSE now() + v.active_offset::interval END,
       now() + v.until_offset::interval,
       CASE WHEN v.release_offset IS NULL THEN NULL ELSE now() + v.release_offset::interval END,
       v.randomize_questions, v.randomize_options,
       CASE WHEN v.attempt_offset IS NULL THEN NULL ELSE now() + v.attempt_offset::interval END,
       u.id
FROM (VALUES
    ('taruna', 'SMA', 'Tryout UTBK SMA', 'Simulasi UTBK dua subtes untuk siswa SMA kelas 12.', 'weighted', 'published', '-7 days', '60 days', NULL, true, true, NULL, 'admin@tarunaeduka.id'),
    ('taruna', 'SMA', 'Tryout SMA Saintek', 'Latihan saintek untuk paket bundle.', 'weighted', 'published', '-5 days', '45 days', NULL, false, true, NULL, 'admin@tarunaeduka.id'),
    ('taruna', 'SMA', 'Tryout SMA Soshum', 'Latihan soshum untuk paket bundle.', 'weighted', 'published', '-5 days', '45 days', NULL, false, false, NULL, 'admin@tarunaeduka.id'),
    ('taruna', 'SMP', 'Tryout SMP Bahasa Inggris', 'Draf tryout bahasa Inggris; belum dipublikasikan.', 'weighted', 'draft', NULL, '45 days', NULL, false, false, NULL, 'admin@tarunaeduka.id'),
    ('taruna', 'SD', 'Tryout SD Matematika', 'Tryout yang sudah dikerjakan satu siswa, jadi terkunci.', 'weighted', 'published', '-14 days', '30 days', '7 days', false, true, '-1 day', 'admin@tarunaeduka.id'),
    ('sekolah-b', 'SMA', 'Tryout SMA Sekolah B', 'Contoh tryout milik tenant kedua.', 'weighted', 'published', '-3 days', '30 days', NULL, false, false, NULL, 'admin@sekolah-b.test')
) AS v (slug, program_code, title, description, scoring_mode, status, active_offset, until_offset,
        release_offset, randomize_questions, randomize_options, attempt_offset, author_email)
JOIN tenants t ON t.slug = v.slug
JOIN programs p ON p.tenant_id = t.id AND p.code = v.program_code
JOIN users u ON u.tenant_id = t.id AND u.email = v.author_email
WHERE NOT EXISTS (SELECT 1 FROM tryouts x WHERE x.tenant_id = t.id AND x.title = v.title);

INSERT INTO tryout_subtests (tryout_id, tenant_id, name, duration_seconds, position)
SELECT x.id, x.tenant_id, v.name, v.duration_seconds, v.position
FROM (VALUES
    ('taruna', 'Tryout UTBK SMA', 'Penalaran Umum', 2700, 1),
    ('taruna', 'Tryout UTBK SMA', 'Pengetahuan Kuantitatif', 1800, 2),
    ('taruna', 'Tryout SMA Saintek', 'Matematika Saintek', 1800, 1),
    ('taruna', 'Tryout SMA Soshum', 'Penalaran Soshum', 1800, 1),
    ('taruna', 'Tryout SMP Bahasa Inggris', 'Tenses', 1200, 1),
    ('taruna', 'Tryout SD Matematika', 'Bilangan', 1800, 1),
    ('sekolah-b', 'Tryout SMA Sekolah B', 'Matematika', 1500, 1)
) AS v (slug, tryout_title, name, duration_seconds, position)
JOIN tenants t ON t.slug = v.slug
JOIN tryouts x ON x.tenant_id = t.id AND x.title = v.tryout_title
WHERE NOT EXISTS (SELECT 1 FROM tryout_subtests s
                   WHERE s.tenant_id = t.id AND s.tryout_id = x.id AND s.name = v.name);

INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id,
        question_version_id)
SELECT s.id, x.id, x.tenant_id, v.position, q.id, qv.id
FROM (VALUES
    ('taruna', 'Tryout UTBK SMA', 'Penalaran Umum', 1, 'Semua siswa yang rajin belajar lulus ujian. Andi lulus ujian. Maka ...'),
    ('taruna', 'Tryout UTBK SMA', 'Penalaran Umum', 2, '2, 6, 12, 20, 30, ... Angka selanjutnya dalam deret tersebut adalah ...'),
    ('taruna', 'Tryout UTBK SMA', 'Penalaran Umum', 3, 'Percepatan gravitasi rata-rata di permukaan bumi adalah 9,8 m/s².'),
    ('taruna', 'Tryout UTBK SMA', 'Pengetahuan Kuantitatif', 1, 'Akar-akar persamaan $x^2 - 5x + 6 = 0$ adalah ...'),
    ('taruna', 'Tryout UTBK SMA', 'Pengetahuan Kuantitatif', 2, 'Nilai dari $\log_2 32$ adalah ...'),
    ('taruna', 'Tryout UTBK SMA', 'Pengetahuan Kuantitatif', 3, 'Turunan pertama dari $f(x) = 3x^2 + 2x$ adalah ...'),
    ('taruna', 'Tryout SMA Saintek', 'Matematika Saintek', 1, 'Turunan pertama dari $f(x) = 3x^2 + 2x$ adalah ...'),
    ('taruna', 'Tryout SMA Saintek', 'Matematika Saintek', 2, 'Nilai dari $\log_2 32$ adalah ...'),
    ('taruna', 'Tryout SMA Soshum', 'Penalaran Soshum', 1, 'Kata ''OPTIMIS'' berlawanan makna (antonim) dengan ...'),
    ('taruna', 'Tryout SMA Soshum', 'Penalaran Soshum', 2, 'Semua siswa yang rajin belajar lulus ujian. Andi lulus ujian. Maka ...'),
    ('taruna', 'Tryout SMP Bahasa Inggris', 'Tenses', 1, 'Jika $2x + 5 = 17$, maka nilai $x$ adalah ...'),
    ('taruna', 'Tryout SMP Bahasa Inggris', 'Tenses', 2, 'Luas permukaan kubus dengan panjang rusuk 6 cm adalah ... $cm^2$'),
    ('taruna', 'Tryout SD Matematika', 'Bilangan', 1, 'Hasil dari 248 + 179 adalah ...'),
    ('taruna', 'Tryout SD Matematika', 'Bilangan', 2, '7 × 8 = ...'),
    ('taruna', 'Tryout SD Matematika', 'Bilangan', 3, 'Keliling persegi dengan panjang sisi 9 cm adalah ... cm'),
    ('sekolah-b', 'Tryout SMA Sekolah B', 'Matematika', 1, 'Hasil dari $3x + 2x$ adalah ...'),
    ('sekolah-b', 'Tryout SMA Sekolah B', 'Matematika', 2, 'Nilai dari $\sin 30^\circ$ adalah ...')
) AS v (slug, tryout_title, subtest_name, position, content)
JOIN tenants t ON t.slug = v.slug
JOIN tryouts x ON x.tenant_id = t.id AND x.title = v.tryout_title
JOIN tryout_subtests s ON s.tryout_id = x.id AND s.name = v.subtest_name
JOIN questions q ON q.tenant_id = t.id AND q.seq = (
    SELECT min(q2.seq) FROM questions q2
      JOIN question_versions v2 ON v2.question_id = q2.id AND v2.version = q2.current_version
     WHERE q2.tenant_id = t.id AND v2.content = v.content)
JOIN question_versions qv ON qv.question_id = q.id AND qv.version = q.current_version
WHERE NOT EXISTS (SELECT 1 FROM tryout_questions tq
                   WHERE tq.tryout_id = x.id AND tq.question_id = q.id);

-- P11: products over the seeded published tryouts; extra SMA tryouts exist so a bundle has two of one program.
INSERT INTO products (tenant_id, type, title, description, program_id, price, original_price, is_active,
        active_until, created_by)
SELECT t.id, v.type, v.title, v.description, p.id, v.price::numeric(14,2),
       v.original_price::numeric(14,2), v.is_active,
       CASE WHEN v.until_offset IS NULL THEN NULL ELSE now() + v.until_offset::interval END, u.id
FROM (VALUES
    ('taruna',    'individual', 'Tryout UTBK SMA',        'Satu tryout UTBK lengkap untuk siswa SMA.',            'SMA', '150000.00', NULL,         true,  NULL,     'admin@tarunaeduka.id'),
    ('taruna',    'individual', 'Tryout SD Matematika',   'Satu tryout matematika SD dengan pembahasan.',         'SD',  '75000.00',  '90000.00',  true,  '30 days', 'admin@tarunaeduka.id'),
    ('taruna',    'bundle',     'Paket Sains SMA',        'Dua tryout saintek dengan harga lebih hemat.',         'SMA', '250000.00', '300000.00', true,  '45 days', 'admin@tarunaeduka.id'),
    ('taruna',    'bundle',     'Paket Lengkap SMA',      'Tiga tryout SMA: UTBK, saintek, dan soshum.',          'SMA', '350000.00', '450000.00', true,  '45 days', 'admin@tarunaeduka.id'),
    ('taruna',    'individual', 'Tryout SMA Soshum',      'Tryout soshum yang belum ditampilkan di katalog.',     'SMA', '100000.00', NULL,        false, NULL,     'admin@tarunaeduka.id'),
    ('sekolah-b', 'individual', 'Tryout SMA Sekolah B',   'Produk contoh milik tenant kedua.',                    'SMA', '125000.00', NULL,        true,  NULL,     'admin@sekolah-b.test')
) AS v (slug, type, title, description, program_code, price, original_price, is_active, until_offset, author_email)
JOIN tenants t ON t.slug = v.slug
JOIN programs p ON p.tenant_id = t.id AND p.code = v.program_code
JOIN users u ON u.tenant_id = t.id AND u.email = v.author_email
WHERE NOT EXISTS (SELECT 1 FROM products x WHERE x.tenant_id = t.id AND lower(x.title) = lower(v.title));

INSERT INTO product_tryouts (product_id, tryout_id, tenant_id, position)
SELECT pr.id, x.id, pr.tenant_id, v.position
FROM (VALUES
    ('taruna',    'Tryout UTBK SMA',      'Tryout UTBK SMA',    1),
    ('taruna',    'Tryout SD Matematika', 'Tryout SD Matematika', 1),
    ('taruna',    'Paket Sains SMA',      'Tryout UTBK SMA',    1),
    ('taruna',    'Paket Sains SMA',      'Tryout SMA Saintek', 2),
    ('taruna',    'Paket Lengkap SMA',    'Tryout UTBK SMA',    1),
    ('taruna',    'Paket Lengkap SMA',    'Tryout SMA Saintek', 2),
    ('taruna',    'Paket Lengkap SMA',    'Tryout SMA Soshum',  3),
    ('taruna',    'Tryout SMA Soshum',    'Tryout SMA Soshum',  1),
    ('sekolah-b', 'Tryout SMA Sekolah B', 'Tryout SMA Sekolah B', 1)
) AS v (slug, product_title, tryout_title, position)
JOIN tenants t ON t.slug = v.slug
JOIN products pr ON pr.tenant_id = t.id AND lower(pr.title) = lower(v.product_title)
JOIN tryouts x ON x.tenant_id = t.id AND x.title = v.tryout_title
WHERE NOT EXISTS (SELECT 1 FROM product_tryouts pt
                   WHERE pt.product_id = pr.id AND pt.tryout_id = x.id);

-- P12: the ledger explains every seeded balance. The opening row is the migration's own statement,
-- here for the seed's users, and the chain below keeps `balance_after` and the projection exact.
INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind, reason,
        source_type, source_id)
SELECT u.tenant_id, u.id, u.balance, u.balance, 'opening', 'system', '', 'opening', u.id::text
FROM users u
WHERE u.tenant_id IS NOT NULL
  AND u.balance <> 0
  AND NOT EXISTS (
      SELECT 1 FROM wallet_entries e
       WHERE e.tenant_id = u.tenant_id
         AND e.source_type = 'opening'
         AND e.source_id = u.id::text);

-- Three extra rows for taruna's Naila: a top-up, a spend and an admin's adjustment, each anchored
-- to the balance of the row before it so re-running the seed changes nothing.
INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind, reason,
        source_type, source_id)
SELECT o.tenant_id, o.user_id, 50000.00, o.balance_after + 50000.00, 'topup', 'provider', '',
       'topup', 'seed-topup-naila'
FROM wallet_entries o
JOIN users s ON s.id = o.user_id AND s.tenant_id = o.tenant_id
JOIN tenants t ON t.id = o.tenant_id
WHERE t.slug = 'taruna' AND s.email = 'naila@example.com' AND o.source_type = 'opening'
  AND NOT EXISTS (SELECT 1 FROM wallet_entries e
                   WHERE e.tenant_id = o.tenant_id AND e.source_type = 'topup'
                     AND e.source_id = 'seed-topup-naila');

INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind, reason,
        source_type, source_id)
SELECT p.tenant_id, p.user_id, -75000.00, p.balance_after - 75000.00, 'spend', 'student', '',
       'order', 'seed-order-naila'
FROM wallet_entries p
WHERE p.source_type = 'topup' AND p.source_id = 'seed-topup-naila'
  AND NOT EXISTS (SELECT 1 FROM wallet_entries e
                   WHERE e.tenant_id = p.tenant_id AND e.source_type = 'order'
                     AND e.source_id = 'seed-order-naila');

INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind, actor_id,
        reason, source_type, source_id)
SELECT p.tenant_id, p.user_id, -5000.00, p.balance_after - 5000.00, 'adjustment', 'staff', a.id,
       'Koreksi saldo awal semester', 'adjustment', 'seed-adjustment-naila'
FROM wallet_entries p
JOIN users a ON a.tenant_id = p.tenant_id AND a.email = 'admin@tarunaeduka.id'
WHERE p.source_type = 'order' AND p.source_id = 'seed-order-naila'
  AND NOT EXISTS (SELECT 1 FROM wallet_entries e
                   WHERE e.tenant_id = p.tenant_id AND e.source_type = 'adjustment'
                     AND e.source_id = 'seed-adjustment-naila');

-- Whatever the chain above did, the projection equals the ledger sum, so the seed can run twice.
UPDATE users u
   SET balance = e.total
  FROM (SELECT tenant_id, user_id, sum(amount) AS total
          FROM wallet_entries GROUP BY tenant_id, user_id) e
 WHERE u.tenant_id = e.tenant_id AND u.id = e.user_id AND u.balance <> e.total;

-- P16: a published, in-window tryout holding all four question types, and an enrollment for the seeded student,
-- so the runtime can be driven from login to result without buying anything first.
INSERT INTO tryouts (tenant_id, program_id, title, description, scoring_mode, status, active_from, active_until,
        randomize_questions, randomize_options, created_by)
SELECT t.id, p.id, 'Tryout Uji Runtime', 'Dua subtes berisi keempat jenis soal untuk menguji pengerjaan.', 'weighted',
       'published', now() - interval '1 day', now() + interval '60 days', true, true, u.id
FROM tenants t
JOIN programs p ON p.tenant_id = t.id AND p.code = 'SD'
JOIN users u ON u.tenant_id = t.id AND u.email = 'admin@tarunaeduka.id'
WHERE t.slug = 'taruna'
  AND NOT EXISTS (SELECT 1 FROM tryouts x WHERE x.tenant_id = t.id AND x.title = 'Tryout Uji Runtime');

INSERT INTO tryout_subtests (tryout_id, tenant_id, name, duration_seconds, position)
SELECT x.id, x.tenant_id, v.name, v.duration_seconds, v.position
FROM (VALUES ('Subtes Pilihan', 600, 1), ('Subtes Isian dan Benar Salah', 600, 2)) AS v (name, duration_seconds, position)
JOIN tenants t ON t.slug = 'taruna'
JOIN tryouts x ON x.tenant_id = t.id AND x.title = 'Tryout Uji Runtime'
WHERE NOT EXISTS (SELECT 1 FROM tryout_subtests s WHERE s.tryout_id = x.id AND s.name = v.name);

INSERT INTO tryout_questions (subtest_id, tryout_id, tenant_id, position, question_id, question_version_id)
SELECT s.id, x.id, x.tenant_id, v.position, q.id, qv.id
FROM (VALUES
    ('Subtes Pilihan', 1, 'Hasil dari 248 + 179 adalah ...'),
    ('Subtes Pilihan', 2, 'Berikut ini yang termasuk sumber energi terbarukan adalah ...'),
    ('Subtes Isian dan Benar Salah', 1, 'Keliling persegi dengan panjang sisi 9 cm adalah ... cm'),
    ('Subtes Isian dan Benar Salah', 2, 'Matahari terbit dari arah timur.')
) AS v (subtest_name, position, content)
JOIN tenants t ON t.slug = 'taruna'
JOIN tryouts x ON x.tenant_id = t.id AND x.title = 'Tryout Uji Runtime'
JOIN tryout_subtests s ON s.tryout_id = x.id AND s.name = v.subtest_name
JOIN questions q ON q.tenant_id = t.id AND q.seq = (
    SELECT min(q2.seq) FROM questions q2
      JOIN question_versions v2 ON v2.question_id = q2.id AND v2.version = q2.current_version
     WHERE q2.tenant_id = t.id AND v2.content = v.content)
JOIN question_versions qv ON qv.question_id = q.id AND qv.version = q.current_version
WHERE NOT EXISTS (SELECT 1 FROM tryout_questions tq WHERE tq.tryout_id = x.id AND tq.question_id = q.id);

INSERT INTO enrollments (tenant_id, user_id, tryout_id)
SELECT t.id, u.id, x.id
FROM tenants t
JOIN users u ON u.tenant_id = t.id AND u.email = 'naila@example.com'
JOIN tryouts x ON x.tenant_id = t.id AND x.title = 'Tryout Uji Runtime'
WHERE t.slug = 'taruna'
ON CONFLICT (tenant_id, user_id, tryout_id) DO NOTHING;
