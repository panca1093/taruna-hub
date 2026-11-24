-- Tryout products sell attempt credits like kecermatan ones; a tryout attempt spends one when it starts.

ALTER TABLE products DROP CONSTRAINT products_kecermatan_check;
UPDATE products SET included_attempts = 1 WHERE included_attempts IS NULL;
ALTER TABLE products
    ALTER COLUMN included_attempts SET NOT NULL,
    ALTER COLUMN included_attempts SET DEFAULT 1,
    ADD CONSTRAINT products_kecermatan_check CHECK ((type = 'kecermatan') = (kecermatan_test_id IS NOT NULL));

ALTER TABLE orders DROP CONSTRAINT orders_kecermatan_check;
UPDATE orders SET included_attempts = 1 WHERE included_attempts IS NULL;
ALTER TABLE orders
    ALTER COLUMN included_attempts SET NOT NULL,
    ALTER COLUMN included_attempts SET DEFAULT 1,
    ADD CONSTRAINT orders_kecermatan_check CHECK ((product_type = 'kecermatan') = (kecermatan_test_id IS NOT NULL));

-- Existing holders keep what they already started and get one attempt if they have not started any.
ALTER TABLE enrollments DROP CONSTRAINT enrollments_credit_check;
UPDATE enrollments e
   SET attempts_used = s.n, attempts_purchased = GREATEST(s.n, 1)
  FROM (SELECT e2.id, count(ts.id)::int AS n
          FROM enrollments e2
          LEFT JOIN tryout_sessions ts
            ON ts.tenant_id = e2.tenant_id AND ts.user_id = e2.user_id AND ts.tryout_id = e2.tryout_id
         WHERE e2.tryout_id IS NOT NULL
         GROUP BY e2.id) s
 WHERE e.id = s.id;
ALTER TABLE enrollments
    ALTER COLUMN attempts_purchased SET NOT NULL,
    ALTER COLUMN attempts_purchased SET DEFAULT 1,
    ADD CONSTRAINT enrollments_credit_check CHECK (attempts_used <= attempts_purchased);
