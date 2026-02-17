-- ==========================================
-- РАСШИРЕНИЯ
-- ==========================================

-- UUID v7 для time-series таблиц (messages, events)
-- Установка: https://github.com/fboulnois/pg_uuidv7
CREATE EXTENSION IF NOT EXISTS pg_uuidv7;

-- ==========================================
-- СИСТЕМА ПРАВ И ДОСТУПА (RBAC)
-- ==========================================

CREATE TABLE permissions
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    slug        VARCHAR(100) UNIQUE NOT NULL, -- Код для проверки в Go: 'chat.delete', 'ads.edit'
    module      VARCHAR(50)         NOT NULL, -- 'chats', 'ads', 'system'
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE roles
(
    id         UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    name       VARCHAR(100)        NOT NULL,           -- 'Администратор', 'Агент'
    slug       VARCHAR(100) UNIQUE NOT NULL,           -- 'admin', 'agent'
    is_system  BOOLEAN                  DEFAULT FALSE, -- TRUE для ролей, которые нельзя удалять
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE role_permissions
(
    role_id       UUID REFERENCES roles (id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users
(
    id         UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    email      VARCHAR(255) UNIQUE NOT NULL,
    full_name  VARCHAR(255),
    status     VARCHAR(50)              DEFAULT 'offline', -- 'online', 'offline', 'busy'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE user_roles
(
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- ==========================================
-- УНИВЕРСАЛЬНЫЙ КАТАЛОГ (ДЛЯ AI И MCP)
-- ==========================================

CREATE TABLE catalog_items
(
    id             UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    entity_type    VARCHAR(50)  NOT NULL,                 -- 'product', 'service', 'property', 'project'
    category       VARCHAR(100),                          -- 'недвижимость', 'ритейл', 'услуги'
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    price          DECIMAL(15, 2),
    currency       VARCHAR(3)               DEFAULT 'USD',
    attributes     JSONB                    DEFAULT '{}', -- Специфичные данные (площадь, бренд, длительность)
    stock_quantity INT,                                   -- NULL для услуг
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ==========================================
-- ТАРГЕТИРОВАННАЯ РЕКЛАМА (ADS LAYER)
-- ==========================================

CREATE TABLE ad_accounts
(
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform            VARCHAR(50)  NOT NULL, -- 'facebook', 'google', 'tiktok'
    external_account_id VARCHAR(255) NOT NULL,
    name                VARCHAR(255),
    access_token        TEXT,
    status              VARCHAR(50),
    UNIQUE (platform, external_account_id)
);

CREATE TABLE ad_campaigns
(
    id            UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    ad_account_id UUID REFERENCES ad_accounts (id) ON DELETE CASCADE,
    external_id   VARCHAR(255) NOT NULL,
    name          VARCHAR(255),
    objective     VARCHAR(50),
    budget_type   VARCHAR(20),
    daily_budget  DECIMAL(15, 2),
    status        VARCHAR(50),
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE ad_groups
(
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id    UUID REFERENCES ad_campaigns (id) ON DELETE CASCADE,
    external_id    VARCHAR(255) NOT NULL,
    name           VARCHAR(255),
    targeting_data JSONB, -- Гео, интересы, возраст
    status         VARCHAR(50)
);

CREATE TABLE ads
(
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ad_group_id  UUID REFERENCES ad_groups (id) ON DELETE CASCADE,
    external_id  VARCHAR(255) NOT NULL,
    name         VARCHAR(255),
    creative_url TEXT,
    status       VARCHAR(50)
);

-- АНАЛИТИКА И ЛИКВИДНОСТЬ РЕКЛАМЫ
CREATE TABLE ad_performance_daily
(
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ad_id            UUID REFERENCES ads (id) ON DELETE CASCADE,
    date             DATE NOT NULL,
    spend            DECIMAL(15, 2)   DEFAULT 0,
    impressions      INT              DEFAULT 0,
    clicks           INT              DEFAULT 0,
    reach            INT              DEFAULT 0,
    video_views_25p  INT              DEFAULT 0,
    video_views_100p INT              DEFAULT 0,
    quality_score    INT,
    UNIQUE (ad_id, date)
);

CREATE TABLE ad_liquidity_reports
(
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id               UUID REFERENCES ad_campaigns (id) ON DELETE CASCADE,
    date                      DATE NOT NULL,
    learning_stage            VARCHAR(50), -- 'learning', 'active', 'limited'
    placement_liquidity_score DECIMAL(3, 2),
    audience_overlap_pct      DECIMAL(5, 2),
    UNIQUE (campaign_id, date)
);

CREATE TABLE ad_lead_events
(
    id               UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    ad_id            UUID  REFERENCES ads (id) ON DELETE SET NULL,
    platform_lead_id VARCHAR(255) UNIQUE,
    raw_data         JSONB NOT NULL,
    processed_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ==========================================
-- ЧАТ-ДВИЖОК (MESSAGING)
-- ==========================================

CREATE TABLE channels
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    platform    VARCHAR(50) NOT NULL, -- 'telegram', 'whatsapp', 'instagram'
    credentials JSONB       NOT NULL,
    is_active   BOOLEAN                  DEFAULT TRUE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE contacts
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    external_id VARCHAR(255) UNIQUE NOT NULL, -- ID в соцсети
    name        VARCHAR(255),
    phone       VARCHAR(50),
    email       VARCHAR(255),                  -- Email контакта (из lead-форм или вручную)
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE dialogs
(
    id               UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    channel_id       UUID REFERENCES channels (id),
    contact_id       UUID REFERENCES contacts (id),
    current_agent_id UUID REFERENCES users (id),
    source_ad_id     UUID REFERENCES ads (id) ON DELETE SET NULL, -- Связка с рекламой
    status           VARCHAR(50)              DEFAULT 'open',     -- 'open', 'pending', 'closed'
    tags             JSONB                    DEFAULT '[]',
    last_message_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE dialog_events
(
    id         UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    dialog_id  UUID REFERENCES dialogs (id) ON DELETE CASCADE,
    -- Тип события: 'assigned', 'transferred', 'status_changed', 'ai_takeover', 'closed'
    event_type VARCHAR(50) NOT NULL,
    -- Кто совершил действие: ID агента или NULL (если действие совершила система/AI)
    actor_id   UUID REFERENCES users (id),
    -- Дополнительные данные (например, 'from_status' -> 'to_status' или 'from_agent' -> 'to_agent')
    payload    JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индекс для быстрого построения таймлайна диалога
CREATE INDEX idx_dialog_events_dialog_id ON dialog_events (dialog_id, created_at);

CREATE TABLE messages
(
    id          UUID PRIMARY KEY         DEFAULT uuid_generate_v7(), -- UUID v7 для лучшей производительности INSERT
    dialog_id   UUID REFERENCES dialogs (id) ON DELETE CASCADE,
    sender_type VARCHAR(50) NOT NULL,                  -- 'customer', 'agent', 'ai', 'system'
    external_id VARCHAR(255),                          -- Platform message ID (mid) для идемпотентности
    content     TEXT,
    payload     JSONB                    DEFAULT '{}', -- Ссылки на медиа (blob), кнопки
    metadata    JSONB                    DEFAULT '{}', -- AI интент, confidence, platform_id
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ==========================================
-- ИНДЕКСЫ ДЛЯ ОПТИМИЗАЦИИ (HIGH LOAD)
-- ==========================================

-- Чат и сообщения
CREATE INDEX idx_messages_dialog_at ON messages (dialog_id, created_at DESC);
CREATE UNIQUE INDEX idx_messages_external_id ON messages (external_id) WHERE external_id IS NOT NULL;
CREATE INDEX idx_dialogs_status ON dialogs (status);
CREATE INDEX idx_contacts_external_id ON contacts (external_id);
CREATE INDEX idx_contacts_email ON contacts (email) WHERE email IS NOT NULL;

-- Реклама
CREATE INDEX idx_ad_performance_date ON ad_performance_daily (date);
CREATE INDEX idx_ads_external_id ON ads (external_id);
CREATE INDEX idx_lead_events_platform_id ON ad_lead_events (platform_lead_id);

-- Каталог
CREATE INDEX idx_catalog_type_category ON catalog_items (entity_type, category);
CREATE INDEX idx_catalog_attributes ON catalog_items USING GIN (attributes);
