-- УНИВЕРСАЛЬНЫЙ КАТАЛОГ (ДЛЯ AI И MCP)

CREATE TABLE catalog_items
(
    id             UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    entity_type    VARCHAR(50)  NOT NULL,
    category       VARCHAR(100),
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    price          DECIMAL(15, 2),
    currency       VARCHAR(3)               DEFAULT 'USD',
    attributes     JSONB                    DEFAULT '{}',
    stock_quantity INT,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);