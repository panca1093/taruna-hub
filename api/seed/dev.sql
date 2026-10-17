-- Dev-only fixtures. Deliberately NOT a migration: production must never run this.
-- Mirrors the mockup's currentStudent so web/ shows the same person before and after cutover.
-- Password hash is bcrypt("password", cost 10), verified with golang.org/x/crypto/bcrypt.
-- Two tenants share naila@example.com on purpose: identifiers are unique per tenant (FR-AUTH-07).
-- Hosts in dev: taruna.localhost, sekolah-b.localhost, admin.localhost (Platform Admin).

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

INSERT INTO users (tenant_id, name, email, phone, password_hash, role, balance)
SELECT t.id, v.name, v.email, v.phone,
       '$2a$10$gZ0w6wuB5MiODdtWAetlPetM3X84pFE04KlyoP3h0Wsoa43WzhEp.', v.role::user_role, v.balance
FROM (VALUES
    ('taruna',    'Naila Ramadhani',  'naila@example.com',       '+6281234567890', 'student',  150000),
    ('taruna',    'Siti Aminah',      'admin@tarunaeduka.id',    '+6281200000001', 'admin',    0),
    ('taruna',    'Rizky Operator',   'operator@tarunaeduka.id', '+6281200000002', 'operator', 0),
    ('sekolah-b', 'Naila Sekolah B',  'naila@example.com',       '+6281234567890', 'student',  25000),
    ('sekolah-b', 'Admin Sekolah B',  'admin@sekolah-b.test',    '+6281200000003', 'admin',    0)
) AS v (slug, name, email, phone, role, balance)
JOIN tenants t ON t.slug = v.slug
ON CONFLICT DO NOTHING;

INSERT INTO users (tenant_id, name, email, phone, password_hash, role)
VALUES (NULL, 'Platform Admin', 'root@tarunaeduka.id', '+6281200000009',
        '$2a$10$gZ0w6wuB5MiODdtWAetlPetM3X84pFE04KlyoP3h0Wsoa43WzhEp.', 'platform_admin')
ON CONFLICT DO NOTHING;
