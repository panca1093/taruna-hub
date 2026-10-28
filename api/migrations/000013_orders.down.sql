DROP TABLE IF EXISTS enrollment_grants;
DROP TABLE IF EXISTS enrollments;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_window_check;
ALTER TABLE products DROP COLUMN IF EXISTS active_from;
