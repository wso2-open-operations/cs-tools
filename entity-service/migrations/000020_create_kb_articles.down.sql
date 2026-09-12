DROP TRIGGER IF EXISTS trg_kb_article_valid_transition ON kb_articles;
DROP FUNCTION IF EXISTS check_kb_article_valid_transition();

DROP TABLE IF EXISTS kb_article_history;
DROP TABLE IF EXISTS kb_articles;
DROP TABLE IF EXISTS kb_managers;
DROP TABLE IF EXISTS knowledge_bases;

DROP TYPE IF EXISTS kb_article_state_enum;

-- Note: pg_trgm extension intentionally left in place -- other tables
-- (e.g. cases.subject) already depend on it for trigram search.
