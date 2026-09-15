-- Whether an article (or, for history, a specific past version of it) was
-- AI-generated, and a free-text label of what generated it. No enum for the
-- label: the source data this mirrors is a loosely-defined choice list, so
-- this is intentionally left unconstrained.
ALTER TABLE kb_articles
    ADD COLUMN generated_with_ai BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_generated_by   TEXT    NULL;

ALTER TABLE kb_article_history
    ADD COLUMN generated_with_ai BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_generated_by   TEXT    NULL;
