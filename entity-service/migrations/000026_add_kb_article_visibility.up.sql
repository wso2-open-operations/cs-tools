-- Whether an article is visible to customers or internal-only (CS engineers).
-- Defaults to internal: the safer default when visibility hasn't been set
-- explicitly, e.g. for articles migrated from an external source.
CREATE TYPE kb_article_visibility_enum AS ENUM (
  'internal',
  'customer_visible'
);

ALTER TABLE kb_articles ADD COLUMN visibility kb_article_visibility_enum NOT NULL DEFAULT 'internal';
