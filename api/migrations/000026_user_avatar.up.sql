-- The avatar a user picked from the built-in set; null means the web app picks one from the name.
ALTER TABLE users ADD COLUMN avatar_key text CHECK (avatar_key ~ '^a(0[1-9]|[12][0-9]|3[0-5])$');
