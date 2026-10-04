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

-- knowledge_base: the article container (ServiceNow's kb_knowledge_base
-- table). kb_managers and product association are deliberately not mapped -
-- no local groups/teams concept to resolve kb_managers against yet, and no
-- reliable KB-to-product derivation exists.
CREATE TABLE IF NOT EXISTS knowledge_base (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    active BOOLEAN
);

-- knowledge_article: one row per ServiceNow kb_knowledge row, including
-- historical versions - there is no separate history table. "History" is
-- just other rows sharing a base_version_id lineage; latest=true marks the
-- current one. state stays VARCHAR rather than a native ENUM per this
-- service's enum-column convention (CLAUDE.md #8): its 7-value set hasn't
-- been through pre-flight verification against production data yet.
CREATE TABLE IF NOT EXISTS knowledge_article (
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
    author_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    revised_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    source_case_id UUID REFERENCES work_item(id) ON DELETE SET NULL,
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

CREATE INDEX IF NOT EXISTS idx_knowledge_article_knowledge_base_id ON knowledge_article (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_article_author_id ON knowledge_article (author_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_article_revised_by_id ON knowledge_article (revised_by_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_article_source_case_id ON knowledge_article (source_case_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_article_base_version_id ON knowledge_article (base_version_id);
