-- Add password_hash column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';

-- Create refresh_tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    user_id     UUID                     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  VARCHAR(64)              NOT NULL,
    device_id   VARCHAR(255)             NOT NULL,
    device_name VARCHAR(255)             NOT NULL DEFAULT '',
    ip_address  VARCHAR(45)              NOT NULL DEFAULT '',
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at  TIMESTAMP WITH TIME ZONE
);

-- Indexes for refresh_tokens
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_device ON refresh_tokens (user_id, device_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens (token_hash) WHERE revoked_at IS NULL;
