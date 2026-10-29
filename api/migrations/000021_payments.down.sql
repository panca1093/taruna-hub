DROP TABLE payment_exceptions;
DROP TABLE payment_events;
DROP TABLE payment_attempts;
ALTER TABLE orders DROP CONSTRAINT orders_payment_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_method_check CHECK (payment_method IN ('manual', 'wallet'));
