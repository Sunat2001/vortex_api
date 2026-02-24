-- Drop refresh_tokens indexes and table
DROP INDEX IF EXISTS idx_refresh_tokens_token_hash;
DROP INDEX IF EXISTS idx_refresh_tokens_user_device;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP TABLE IF EXISTS refresh_tokens;

-- Remove password_hash column from users
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
