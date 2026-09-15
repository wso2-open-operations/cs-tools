ALTER TABLE kb_articles ADD COLUMN updated_by TEXT NULL REFERENCES users(id);
