-- ИНДЕКСЫ ДЛЯ ОПТИМИЗАЦИИ (HIGH LOAD)

-- Чат и сообщения
CREATE INDEX idx_dialog_events_dialog_id ON dialog_events (dialog_id, created_at);
CREATE INDEX idx_messages_dialog_at ON messages (dialog_id, created_at DESC);
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