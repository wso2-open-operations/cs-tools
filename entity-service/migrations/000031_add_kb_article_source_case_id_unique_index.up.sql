CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_article_source_case_id_unique
	ON knowledge_article (source_case_id)
	WHERE source_case_id IS NOT NULL;
