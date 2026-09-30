CREATE OR REPLACE FUNCTION check_kb_article_knowledge_base_active()
RETURNS TRIGGER AS $$
DECLARE
  kb_active BOOLEAN;
BEGIN
  SELECT is_active INTO kb_active FROM knowledge_bases WHERE id = NEW.knowledge_base_id;
  IF kb_active IS FALSE THEN
    RAISE EXCEPTION 'Cannot create a KB article under a deactivated knowledge base';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_kb_article_knowledge_base_active
  BEFORE INSERT ON kb_articles
  FOR EACH ROW EXECUTE FUNCTION check_kb_article_knowledge_base_active();
