-- ЧАТ-ДВИЖОК (MESSAGING)

CREATE TABLE channels
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    platform    VARCHAR(50) NOT NULL,
    credentials JSONB       NOT NULL,
    is_active   BOOLEAN                  DEFAULT TRUE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE contacts
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    external_id VARCHAR(255) UNIQUE NOT NULL,
    name        VARCHAR(255),
    phone       VARCHAR(50),
    email       VARCHAR(255),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE dialogs
(
    id               UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    channel_id       UUID REFERENCES channels (id),
    contact_id       UUID REFERENCES contacts (id),
    current_agent_id UUID REFERENCES users (id),
    source_ad_id     UUID REFERENCES ads (id) ON DELETE SET NULL,
    status           VARCHAR(50)              DEFAULT 'open',
    tags             JSONB                    DEFAULT '[]',
    last_message_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE dialog_events
(
    id         UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    dialog_id  UUID REFERENCES dialogs (id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    actor_id   UUID REFERENCES users (id),
    payload    JSONB                    DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Messages table: uses uuid_generate_v7() if pg_uuidv7 is installed, otherwise gen_random_uuid()
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_uuidv7') THEN
        EXECUTE '
            CREATE TABLE messages
            (
                id          UUID PRIMARY KEY         DEFAULT uuid_generate_v7(),
                dialog_id   UUID REFERENCES dialogs (id) ON DELETE CASCADE,
                sender_type VARCHAR(50) NOT NULL,
                content     TEXT,
                payload     JSONB                    DEFAULT ''{}'',
                metadata    JSONB                    DEFAULT ''{}'',
                created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
            )';
        RAISE NOTICE 'Created messages table with UUID v7';
    ELSE
        EXECUTE '
            CREATE TABLE messages
            (
                id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
                dialog_id   UUID REFERENCES dialogs (id) ON DELETE CASCADE,
                sender_type VARCHAR(50) NOT NULL,
                content     TEXT,
                payload     JSONB                    DEFAULT ''{}'',
                metadata    JSONB                    DEFAULT ''{}'',
                created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
            )';
        RAISE NOTICE 'Created messages table with UUID v4 (pg_uuidv7 not available)';
    END IF;
END $$;