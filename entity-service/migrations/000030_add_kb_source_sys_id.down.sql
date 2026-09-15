DROP INDEX IF EXISTS idx_kb_article_history_source_sys_id;
DROP INDEX IF EXISTS idx_kb_articles_source_sys_id;

ALTER TABLE kb_article_history DROP COLUMN source_sys_id;
ALTER TABLE kb_articles DROP COLUMN source_sys_id;
