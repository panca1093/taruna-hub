ALTER TABLE enrollments
    DROP CONSTRAINT enrollments_credit_check,
    ALTER COLUMN attempts_purchased DROP NOT NULL,
    ALTER COLUMN attempts_purchased DROP DEFAULT;
UPDATE enrollments SET attempts_purchased = NULL, attempts_used = 0 WHERE kecermatan_test_id IS NULL;
ALTER TABLE enrollments ADD CONSTRAINT enrollments_credit_check CHECK (
    (kecermatan_test_id IS NULL AND attempts_purchased IS NULL)
    OR (kecermatan_test_id IS NOT NULL AND attempts_purchased IS NOT NULL AND attempts_used <= attempts_purchased));

ALTER TABLE orders
    DROP CONSTRAINT orders_kecermatan_check,
    ALTER COLUMN included_attempts DROP NOT NULL,
    ALTER COLUMN included_attempts DROP DEFAULT;
UPDATE orders SET included_attempts = NULL WHERE product_type <> 'kecermatan';
ALTER TABLE orders ADD CONSTRAINT orders_kecermatan_check CHECK (
    (product_type = 'kecermatan' AND kecermatan_test_id IS NOT NULL AND included_attempts IS NOT NULL)
    OR (product_type <> 'kecermatan' AND kecermatan_test_id IS NULL AND included_attempts IS NULL));

ALTER TABLE products
    DROP CONSTRAINT products_kecermatan_check,
    ALTER COLUMN included_attempts DROP NOT NULL,
    ALTER COLUMN included_attempts DROP DEFAULT;
UPDATE products SET included_attempts = NULL WHERE type <> 'kecermatan';
ALTER TABLE products ADD CONSTRAINT products_kecermatan_check CHECK (
    (type = 'kecermatan' AND kecermatan_test_id IS NOT NULL AND included_attempts IS NOT NULL)
    OR (type <> 'kecermatan' AND kecermatan_test_id IS NULL AND included_attempts IS NULL));
