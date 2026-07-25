-- Dev-only fixtures. Deliberately NOT a migration: production must never run this.
-- Mirrors the mockup's currentStudent so web/ shows the same person before and after cutover.
-- Password hash is bcrypt("password").

INSERT INTO programs (code, name) VALUES
    ('SD', 'Sekolah Dasar'),
    ('SMP', 'Sekolah Menengah Pertama'),
    ('SMA', 'Sekolah Menengah Atas'),
    ('CASN', 'Calon Aparatur Sipil Negara'),
    ('BUMN', 'Badan Usaha Milik Negara'),
    ('AKPOL', 'Akademi Kepolisian')
ON CONFLICT (code) DO NOTHING;

INSERT INTO users (name, email, phone, password_hash, role, balance) VALUES
    ('Naila Ramadhani', 'naila@example.com', '081234567890',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'student', 150000),
    ('Siti Aminah', 'admin@tarunaeduka.id', '081200000001',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', 0),
    ('Rizky Operator', 'operator@tarunaeduka.id', '081200000002',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'operator', 0)
ON CONFLICT (email) DO NOTHING;
