ALTER TABLE kb_article_history
    DROP COLUMN generated_with_ai,
    DROP COLUMN ai_generated_by;

ALTER TABLE kb_articles
    DROP COLUMN generated_with_ai,
    DROP COLUMN ai_generated_by;
