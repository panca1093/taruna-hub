-- Both feed the automatic avatar; null on accounts made before sign-up asked for them, and on staff.
ALTER TABLE users
    ADD COLUMN gender text CHECK (gender IN ('male', 'female')),
    ADD COLUMN birth_date date;
