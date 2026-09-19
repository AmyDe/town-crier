-- +goose Up
ALTER TABLE users ADD COLUMN lifetime_tier text NOT NULL DEFAULT 'Free';
ALTER TABLE users ADD COLUMN lifetime_original_transaction_id text;
ALTER TABLE users ADD COLUMN lifetime_purchased_at timestamptz;
ALTER TABLE users ADD COLUMN subscription_product_id text;
CREATE INDEX users_lifetime_original_transaction_id ON users (lifetime_original_transaction_id);

-- +goose Down
DROP INDEX IF EXISTS users_lifetime_original_transaction_id;
ALTER TABLE users DROP COLUMN IF EXISTS subscription_product_id;
ALTER TABLE users DROP COLUMN IF EXISTS lifetime_purchased_at;
ALTER TABLE users DROP COLUMN IF EXISTS lifetime_original_transaction_id;
ALTER TABLE users DROP COLUMN IF EXISTS lifetime_tier;
