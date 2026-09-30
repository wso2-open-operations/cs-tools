-- Fixes a race condition (CodeRabbit finding): the previous version of this
-- trigger (000023) read knowledge_bases.is_active without locking the row,
-- so a concurrent deactivation could slip in between the check and the
-- INSERT completing. It also only fired on INSERT, so moving an existing
-- article to a different (possibly deactivated) knowledge_base_id via
-- UPDATE bypassed the check entirely.
--
-- This version: (1) locks the knowledge_bases row with FOR SHARE so a
-- concurrent UPDATE to is_active must wait until this transaction
-- commits/rolls back, closing the check-then-act gap; (2) fires on UPDATE
-- OF knowledge_base_id too, so reassigning an article's knowledge base is
-- validated the same way creating it is.
CREATE OR REPLACE FUNCTION check_kb_article_knowledge_base_active()
RETURNS TRIGGER AS $$
DECLARE
  kb_active BOOLEAN;
BEGIN
  SELECT is_active INTO kb_active
    FROM knowledge_bases
   WHERE id = NEW.knowledge_base_id
   FOR SHARE;

  IF kb_active IS FALSE THEN
    RAISE EXCEPTION 'Cannot create or move a KB article to a deactivated knowledge base';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_kb_article_knowledge_base_active ON kb_articles;

CREATE TRIGGER trg_kb_article_knowledge_base_active
  BEFORE INSERT OR UPDATE OF knowledge_base_id ON kb_articles
  FOR EACH ROW EXECUTE FUNCTION check_kb_article_knowledge_base_active();
