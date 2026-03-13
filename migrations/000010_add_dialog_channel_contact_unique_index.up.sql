CREATE UNIQUE INDEX IF NOT EXISTS idx_dialogs_channel_contact_active
    ON dialogs (channel_id, contact_id)
    WHERE status IN ('open', 'pending');
