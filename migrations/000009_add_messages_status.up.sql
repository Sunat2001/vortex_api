BEGIN;

ALTER TABLE messages
    ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'sent';

ALTER TABLE messages
    ADD CONSTRAINT chk_messages_status
        CHECK (status IN ('sent', 'delivered', 'read', 'failed'));

-- Partial index: only working states for retry/monitoring queries
CREATE INDEX idx_messages_status ON messages (status)
    WHERE status IN ('sent', 'failed');

COMMIT;
