-- Per-article usage counters. Current, cumulative values only -- not tracked
-- in kb_article_history since a count isn't meaningfully tied to a version.
ALTER TABLE kb_articles
    ADD COLUMN helpful_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN rating        NUMERIC NULL,
    ADD COLUMN use_count     INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN view_count    INTEGER NOT NULL DEFAULT 0;
