DROP TRIGGER kecermatan_results_counts_immutable ON kecermatan_results;
DROP FUNCTION kecermatan_results_counts_immutable();
DROP TABLE kecermatan_results;
DROP TABLE kecermatan_session_answers;
DROP TABLE kecermatan_session_kolom;
DROP TABLE kecermatan_sessions;

DROP INDEX enrollments_kecermatan_idx;
DELETE FROM enrollment_grants WHERE enrollment_id IN (SELECT id FROM enrollments WHERE kecermatan_test_id IS NOT NULL);
DELETE FROM enrollments WHERE kecermatan_test_id IS NOT NULL;
ALTER TABLE enrollments
    DROP CONSTRAINT enrollments_kecermatan_test_fkey,
    DROP CONSTRAINT enrollments_kecermatan_holder_key,
    DROP CONSTRAINT enrollments_credit_check,
    DROP CONSTRAINT enrollments_kind_check,
    DROP COLUMN kecermatan_test_id,
    ALTER COLUMN tryout_id SET NOT NULL;

DELETE FROM orders WHERE product_type = 'kecermatan';
ALTER TABLE orders
    DROP CONSTRAINT orders_kecermatan_test_fkey,
    DROP CONSTRAINT orders_kecermatan_check,
    DROP COLUMN included_attempts,
    DROP COLUMN kecermatan_test_id,
    DROP CONSTRAINT orders_product_type_check,
    ADD CONSTRAINT orders_product_type_check CHECK (product_type IN ('individual', 'bundle'));

DROP INDEX products_kecermatan_test_idx;
DELETE FROM products WHERE type = 'kecermatan';
ALTER TABLE products
    DROP CONSTRAINT products_kecermatan_test_fkey,
    DROP CONSTRAINT products_kecermatan_check,
    DROP COLUMN included_attempts,
    DROP COLUMN kecermatan_test_id,
    DROP CONSTRAINT products_type_check,
    ADD CONSTRAINT products_type_check CHECK (type IN ('individual', 'bundle'));
