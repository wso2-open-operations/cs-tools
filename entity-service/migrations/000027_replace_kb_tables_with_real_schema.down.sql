-- LOCAL-ONLY simplified rollback: drops the real-schema KB tables. Does
-- NOT recreate the original kb_articles/knowledge_bases/kb_managers/
-- kb_article_history schema (that design is superseded, not worth
-- restoring) -- if you need to roll back, re-run migrations 000020-000026
-- from scratch on a fresh local DB instead.
DROP TABLE IF EXISTS knowledge_base_manager_group;
DROP TABLE IF EXISTS knowledge_base_manager_user;
DROP TABLE IF EXISTS knowledge_article;
DROP TABLE IF EXISTS knowledge_base;
