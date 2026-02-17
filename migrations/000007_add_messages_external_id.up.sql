ALTER TABLE messages ADD COLUMN external_id VARCHAR(255);

CREATE UNIQUE INDEX idx_messages_external_id ON messages (external_id) WHERE external_id IS NOT NULL;