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
-- and compose stack stopped on it. The rest of what it made is 0044's:
-- 000027 dropped everything 000020-000026 created, its knowledge_base is
-- 0044's, and its knowledge_article is 0044's without the foreign keys on
-- author_id, revised_by_id and source_case_id (see entity-service/CLAUDE.md).
-- csm-sync-service does not map knowledge base managers, so these two tables
-- are entity-service's alone.
--
-- user_id references "user". 000027 left that foreign key out, and its header
-- says why: "the real tables have FKs on author_id/revised_by_id/user_id ->
-- "user"(id)", omitted only because the local database of the time had a
-- plural users table (git show 67f35215a:entity-service/migrations/
-- 000027_replace_kb_tables_with_real_schema.up.sql). The knowledge_article
-- half of that statement matches csm-sync-service's 0044, which carries those
-- foreign keys. 000020 cannot run on a database the sync built, so a deployed
-- database cannot have the tables from 000027 through `make migrate`. Dev,
-- staging and production were not inspected. The real constraint's delete
-- action is not recorded; CASCADE is used here, as for knowledge_base_id
-- (user_id is NOT NULL, so SET NULL is impossible).
-- group_id has no foreign key: groups are not rows here (group_name is kept
-- beside the id), as in 000027.
--
-- Fresh databases get the tables with the foreign key. Where the tables
-- already exist, nothing is dropped, rewritten or deleted: a database whose
-- manager user table has no foreign key from user_id to "user" (one that ran
-- 000027) gets it added NOT VALID, so every new row is checked, and then
-- validated. If existing rows name a user that does not exist, validation is
-- skipped with a NOTICE and the rows are kept; delete or correct them, then
-- run ALTER TABLE knowledge_base_manager_user VALIDATE CONSTRAINT
-- knowledge_base_manager_user_user_id_fkey. A foreign key that is already
-- there, whatever its name, is left as it is. Needs knowledge_base (0044) and
-- "user" (0002).

CREATE TABLE IF NOT EXISTS knowledge_base_manager_user (
    id UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
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

DO $$
DECLARE
    orphans BIGINT;
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint con
          JOIN pg_attribute att
            ON att.attrelid = con.conrelid AND att.attnum = ANY (con.conkey)
         WHERE con.conrelid = 'knowledge_base_manager_user'::regclass
           AND con.contype = 'f'
           AND con.confrelid = '"user"'::regclass
           AND cardinality(con.conkey) = 1
           AND att.attname = 'user_id'
    ) THEN
        RETURN;
    END IF;

    ALTER TABLE knowledge_base_manager_user
        ADD CONSTRAINT knowledge_base_manager_user_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE NOT VALID;

    SELECT count(*) INTO orphans
      FROM knowledge_base_manager_user m
     WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = m.user_id);

    IF orphans > 0 THEN
        RAISE NOTICE 'knowledge_base_manager_user: % row(s) name a user that does not exist; knowledge_base_manager_user_user_id_fkey stays NOT VALID until they are deleted or corrected', orphans;
        RETURN;
    END IF;

    ALTER TABLE knowledge_base_manager_user
        VALIDATE CONSTRAINT knowledge_base_manager_user_user_id_fkey;
END
$$;
