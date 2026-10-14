-- Dev-only fixtures. Deliberately NOT a migration: production must never run this.
-- Mirrors the mockup's currentStudent so web/ shows the same person before and after cutover.
-- Password hash is bcrypt("password", cost 10), verified with golang.org/x/crypto/bcrypt.
-- Two tenants share naila@example.com on purpose: identifiers are unique per tenant (FR-AUTH-07).
-- Hosts in dev: taruna.localhost, sekolah-b.localhost, admin.localhost (Platform Admin).

INSERT INTO programs (code, name) VALUES
    ('SD', 'Sekolah Dasar'),
    ('SMP', 'Sekolah Menengah Pertama'),
    ('SMA', 'Sekolah Menengah Atas'),
    ('CASN', 'Calon Aparatur Sipil Negara'),
    ('BUMN', 'Badan Usaha Milik Negara'),
    ('AKPOL', 'Akademi Kepolisian')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, name) VALUES
    ('taruna', 'Taruna Eduka'),
    ('sekolah-b', 'Sekolah B')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO users (tenant_id, name, email, phone, password_hash, role, balance)
SELECT t.id, v.name, v.email, v.phone,
       '$2a$10$gZ0w6wuB5MiODdtWAetlPetM3X84pFE04KlyoP3h0Wsoa43WzhEp.', v.role::user_role, v.balance
FROM (VALUES
    ('taruna',    'Naila Ramadhani',  'naila@example.com',       '081234567890', 'student',  150000),
    ('taruna',    'Siti Aminah',      'admin@tarunaeduka.id',    '081200000001', 'admin',    0),
    ('taruna',    'Rizky Operator',   'operator@tarunaeduka.id', '081200000002', 'operator', 0),
    ('sekolah-b', 'Naila Sekolah B',  'naila@example.com',       '081234567890', 'student',  25000),
    ('sekolah-b', 'Admin Sekolah B',  'admin@sekolah-b.test',    '081200000003', 'admin',    0)
) AS v (slug, name, email, phone, role, balance)
JOIN tenants t ON t.slug = v.slug
ON CONFLICT DO NOTHING;

INSERT INTO users (tenant_id, name, email, phone, password_hash, role)
VALUES (NULL, 'Platform Admin', 'root@tarunaeduka.id', '081200000009',
        '$2a$10$gZ0w6wuB5MiODdtWAetlPetM3X84pFE04KlyoP3h0Wsoa43WzhEp.', 'platform_admin')
ON CONFLICT DO NOTHING;
