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

-- knowledge_base_manager_user / knowledge_base_manager_group: who may manage
-- the articles of a knowledge base, individual users and groups
-- (kb_manager_repo.go, POST /kb-manager-users, POST /kb-manager-groups).
--
-- Both tables used to be created only by the old-style pair
-- 000027_replace_kb_tables_with_real_schema.up.sql / .down.sql, the end of the
-- 000020-000027 chain, which this change removes. That chain could not run on
-- a database built from these migrations: 000020 references products, users
-- and cases, tables this schema has never had, so every fresh `make migrate`
-- and compose stack stopped on it. Nothing else in the chain survives it:
-- 000027 dropped everything 000020-000026 created, and its knowledge_base and
-- knowledge_article are 0044's (the shared table, which csm-sync-service also
-- creates; it does not map knowledge base managers, so these two tables are
-- entity-service's alone).
--
-- Same definitions as 000027, every statement IF NOT EXISTS, and nothing that
-- drops, alters or writes: a no-op wherever 000027 already ran, so a database
-- that has 000020-000027 recorded is left as it is. user_id and group_id keep
-- 000027's shape (no foreign key), so a fresh database and an old one end up
-- with the same tables. Needs knowledge_base (0044).

CREATE TABLE IF NOT EXISTS knowledge_base_manager_user (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_kb_manager_user UNIQUE (knowledge_base_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_kb_manager_user_knowledge_base_id ON knowledge_base_manager_user (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_kb_manager_user_user_id ON knowledge_base_manager_user (user_id);

CREATE TABLE IF NOT EXISTS knowledge_base_manager_group (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    group_id UUID NOT NULL,
    group_name VARCHAR(255),
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_kb_manager_group UNIQUE (knowledge_base_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_kb_manager_group_knowledge_base_id ON knowledge_base_manager_group (knowledge_base_id);
