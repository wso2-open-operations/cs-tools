-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- knowledge_article_history: one snapshot (title, body, state, who changed it) per KB article state change. UpdateKBArticleState inserts a row in the same
-- transaction as the state change, and GET /kb-articles/{id}/history reads them back, so an article's "Version N" and its history page come from here.
--
-- This replaces the old-style pair 000030_knowledge_article_history.up.sql / .down.sql, which are removed in the same change. That pair was never part of
-- the numbered series the environments are built from, so the table only existed where someone created it by hand (as on staging). It was also unsafe to
-- leave in place: `make migrate` runs every untracked *.sql file in name order, the .down.sql included, and that one is DROP TABLE.
--
-- Safe to run anywhere, any number of times: it creates the table and its index only if they are missing, and never drops, truncates, alters or rewrites
-- anything. A database that already has the table, and its rows, whether it was created by hand or by an earlier run of this file, is left exactly as it
-- is. That includes a database that is refilled by a ServiceNow to Postgres data migration: nothing here removes or recreates an existing table.
-- The definition is the one the removed 000030 file used, which is the shape the repository code reads and writes.

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
