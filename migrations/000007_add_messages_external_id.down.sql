DROP INDEX IF EXISTS idx_messages_external_id;

ALTER TABLE messages DROP COLUMN IF EXISTS external_id;