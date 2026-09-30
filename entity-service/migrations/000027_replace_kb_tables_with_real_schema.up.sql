-- Replaces the original local KB tables (kb_articles, knowledge_bases,
-- kb_managers, kb_article_history) with the REAL shared-DB schema Sajith
-- confirmed (kb-tables.sql, Sep 22). Two structural changes from the
-- original design: (1) reviewer permissions split into two tables --
-- individual users AND groups, instead of one; (2) no separate history
-- table -- edits create new knowledge_article rows linked via
-- base_version_id, with latest=true marking the current version.
--
-- LOCAL-ONLY SIMPLIFICATION: the real tables have FKs on author_id/
-- revised_by_id/user_id -> "user"(id) and source_case_id -> work_item(id).
-- This local DB still has the old "users" (plural) table and no work_item
-- table at all, so those FKs are deliberately omitted here (plain UUID
-- columns, no REFERENCES) -- app code already validates IDs before
-- insert, so this doesn't affect correctness of anything being tested.
-- knowledge_base_id and base_version_id DO keep their FKs, since
-- knowledge_base/knowledge_article both exist locally.

DROP TRIGGER IF EXISTS trg_kb_article_valid_transition ON kb_articles;
DROP TRIGGER IF EXISTS trg_kb_article_knowledge_base_active ON kb_articles;
DROP FUNCTION IF EXISTS check_kb_article_valid_transition();
DROP FUNCTION IF EXISTS check_kb_article_knowledge_base_active();

DROP TABLE IF EXISTS kb_article_history;
DROP TABLE IF EXISTS kb_managers;
DROP TABLE IF EXISTS kb_articles;
DROP TABLE IF EXISTS knowledge_bases;
DROP TYPE IF EXISTS kb_article_state_enum;

CREATE TABLE knowledge_base (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    active BOOLEAN
);

CREATE TABLE knowledge_article (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(100),
    title VARCHAR(255) NOT NULL,
    body TEXT,
    state VARCHAR(40),
    knowledge_base_id UUID REFERENCES knowledge_base(id) ON DELETE SET NULL,
    author_id UUID,       -- LOCAL: no FK to "user" -- see file header
    revised_by_id UUID,   -- LOCAL: no FK to "user" -- see file header
    source_case_id UUID,  -- LOCAL: no FK to work_item -- see file header
    base_version_id UUID REFERENCES knowledge_article(id) ON DELETE SET NULL,
    latest BOOLEAN,
    rejection_comment TEXT,
    generated_with_ai BOOLEAN,
    ai_generated_by VARCHAR(100),
    helpful_count INTEGER,
    rating NUMERIC,
    use_count INTEGER,
    view_count INTEGER,
    published_on TIMESTAMPTZ,
    retired_on TIMESTAMPTZ,
    scheduled_publish_on TIMESTAMPTZ
);

CREATE INDEX idx_knowledge_article_knowledge_base_id ON knowledge_article (knowledge_base_id);
CREATE INDEX idx_knowledge_article_author_id ON knowledge_article (author_id);
CREATE INDEX idx_knowledge_article_revised_by_id ON knowledge_article (revised_by_id);
CREATE INDEX idx_knowledge_article_source_case_id ON knowledge_article (source_case_id);
CREATE INDEX idx_knowledge_article_base_version_id ON knowledge_article (base_version_id);

CREATE TABLE knowledge_base_manager_user (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,  -- LOCAL: no FK to "user" -- see file header
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_kb_manager_user UNIQUE (knowledge_base_id, user_id)
);

CREATE INDEX idx_kb_manager_user_knowledge_base_id ON knowledge_base_manager_user (knowledge_base_id);
CREATE INDEX idx_kb_manager_user_user_id ON knowledge_base_manager_user (user_id);

CREATE TABLE knowledge_base_manager_group (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    group_id UUID NOT NULL,
    group_name VARCHAR(255),
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_kb_manager_group UNIQUE (knowledge_base_id, group_id)
);

CREATE INDEX idx_kb_manager_group_knowledge_base_id ON knowledge_base_manager_group (knowledge_base_id);
