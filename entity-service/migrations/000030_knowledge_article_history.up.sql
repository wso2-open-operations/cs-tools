-- Real KB article transition-history table, confirmed needed by Sajith
-- (Sep 23) after the earlier finding that the schema had no way to track
-- who submitted/approved/rejected an article and when. One row per state
-- transition, each carrying a content snapshot (title/body) at that point
-- so consecutive rows can be diffed. Matches the frontend's already-built
-- KBArticleHistoryEntry contract (id, kbArticleId, title, body, state,
-- changedBy, createdOn) exactly -- no frontend changes needed once this
-- exists and is wired up on the Go side.
--
-- Articles migrated from ServiceNow will have no rows here (SN never
-- tracked this) -- expected and fine per Sajith, not a bug. Every new
-- article going forward gets full history from creation onward.
CREATE TABLE IF NOT EXISTS knowledge_article_history (
    id UUID PRIMARY KEY,
    knowledge_article_id UUID NOT NULL REFERENCES knowledge_article(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    state TEXT NOT NULL,
    changed_by UUID NOT NULL REFERENCES "user"(id),
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_knowledge_article_history_article_id ON knowledge_article_history (knowledge_article_id);
