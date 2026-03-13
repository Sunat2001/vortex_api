BEGIN;

DROP INDEX IF EXISTS idx_messages_status;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS chk_messages_status;

ALTER TABLE messages
    DROP COLUMN IF EXISTS status;

COMMIT;
