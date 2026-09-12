CREATE TYPE kb_article_state_enum AS ENUM (
  'draft',
  'pending_review',
  'published',
  'retired'
);

-- One knowledge base per product (API Manager, Identity Server, MI, Open Banking, etc).
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID        NOT NULL UNIQUE REFERENCES products(id),
    name        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_product_id ON knowledge_bases (product_id);

-- Ownership/access grants: which users may review, approve, reject, or edit
-- articles submitted under a given knowledge base. Many-to-many: a KB may
-- have several owners, and one user may own several KBs.
CREATE TABLE IF NOT EXISTS kb_managers (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id  UUID        NOT NULL REFERENCES knowledge_bases(id),
    user_id            TEXT        NOT NULL REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_kb_managers_kb_user UNIQUE (knowledge_base_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_kb_managers_knowledge_base_id ON kb_managers (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_kb_managers_user_id           ON kb_managers (user_id);

CREATE TABLE IF NOT EXISTS kb_articles (
    id                  UUID                   PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id   UUID                   NOT NULL REFERENCES knowledge_bases(id),
    title               TEXT                   NOT NULL,
    body                TEXT                   NOT NULL,
    state               kb_article_state_enum  NOT NULL DEFAULT 'draft',
    author_id           TEXT                   NOT NULL REFERENCES users(id),
    reviewer_id         TEXT                   NULL REFERENCES users(id),

    -- Optional: set only when this article originated from a case-closure
    -- auto-draft (future phase). NULL for manually created articles.
    source_case_id      UUID                   NULL REFERENCES cases(id),

    rejection_comment   TEXT                   NULL,

    created_at          TIMESTAMPTZ            NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ            NOT NULL DEFAULT NOW(),
    submitted_at        TIMESTAMPTZ,
    published_at        TIMESTAMPTZ,
    retired_at          TIMESTAMPTZ,

    CONSTRAINT chk_submitted_at_on_or_after_pending
      CHECK (state NOT IN ('pending_review', 'published', 'retired') OR submitted_at IS NOT NULL),

    CONSTRAINT chk_published_at_on_published
      CHECK (state != 'published' OR published_at IS NOT NULL),

    CONSTRAINT chk_retired_at_on_retired
      CHECK (state != 'retired' OR retired_at IS NOT NULL),

    CONSTRAINT chk_rejection_comment_only_after_reject
      CHECK (rejection_comment IS NULL OR state = 'draft')
);

-- Enforce legal state transitions at the DB level, independent of app logic.
CREATE OR REPLACE FUNCTION check_kb_article_valid_transition()
RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND OLD.state != NEW.state THEN
    IF NOT (
      (OLD.state = 'draft'           AND NEW.state = 'pending_review')
      OR (OLD.state = 'pending_review' AND NEW.state IN ('draft', 'published'))
      OR (OLD.state = 'published'      AND NEW.state IN ('draft', 'retired'))
    ) THEN
      RAISE EXCEPTION 'Invalid KB article state transition: % -> %', OLD.state, NEW.state;
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_kb_article_valid_transition
  BEFORE UPDATE OF state ON kb_articles
  FOR EACH ROW EXECUTE FUNCTION check_kb_article_valid_transition();

-- FK / equality indexes
CREATE INDEX IF NOT EXISTS idx_kb_articles_knowledge_base_id ON kb_articles (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_kb_articles_author_id         ON kb_articles (author_id);
CREATE INDEX IF NOT EXISTS idx_kb_articles_reviewer_id       ON kb_articles (reviewer_id) WHERE reviewer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_articles_source_case_id    ON kb_articles (source_case_id) WHERE source_case_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_articles_state             ON kb_articles (state);

-- Composite index for the common "list articles in this KB, filtered by state" query.
CREATE INDEX IF NOT EXISTS idx_kb_articles_kb_state ON kb_articles (knowledge_base_id, state);

-- Trigram index for title search, matching the pattern used on cases.subject.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_kb_articles_title_trgm ON kb_articles USING GIN (title gin_trgm_ops);

-- Append-only content/state snapshots, written at meaningful transitions
-- (not on every autosave) so an approver can see what changed.
CREATE TABLE IF NOT EXISTS kb_article_history (
    id             UUID                   PRIMARY KEY DEFAULT gen_random_uuid(),
    kb_article_id  UUID                   NOT NULL REFERENCES kb_articles(id),
    title          TEXT                   NOT NULL,
    body           TEXT                   NOT NULL,
    state          kb_article_state_enum  NOT NULL,
    changed_by     TEXT                   NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ            NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kb_article_history_kb_article_id ON kb_article_history (kb_article_id);
CREATE INDEX IF NOT EXISTS idx_kb_article_history_kb_article_time ON kb_article_history (kb_article_id, created_at DESC);
