-- ТАРГЕТИРОВАННАЯ РЕКЛАМА (ADS LAYER)

CREATE TABLE ad_accounts
(
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform            VARCHAR(50)  NOT NULL,
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
    targeting_data JSONB,
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
    learning_stage            VARCHAR(50),
    placement_liquidity_score DECIMAL(3, 2),
    audience_overlap_pct      DECIMAL(5, 2),
    UNIQUE (campaign_id, date)
);

CREATE TABLE ad_lead_events
(
    id               UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    ad_id            UUID REFERENCES ads (id) ON DELETE SET NULL,
    platform_lead_id VARCHAR(255) UNIQUE,
    raw_data         JSONB NOT NULL,
    processed_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);