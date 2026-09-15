-- Origin-system identifier for rows migrated from an external knowledge
-- source (e.g. a ServiceNow kb_knowledge sys_id). Nullable: only set on rows
-- that were backfilled from that source; rows created natively in this
-- platform leave it NULL. Indexed (partial, non-NULL only) so a backfill
-- tool can cheaply check "have I already migrated this row" for idempotency.
ALTER TABLE kb_articles
    ADD COLUMN source_sys_id TEXT NULL;

ALTER TABLE kb_article_history
    ADD COLUMN source_sys_id TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_kb_articles_source_sys_id
    ON kb_articles (source_sys_id) WHERE source_sys_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_kb_article_history_source_sys_id
    ON kb_article_history (source_sys_id) WHERE source_sys_id IS NOT NULL;
