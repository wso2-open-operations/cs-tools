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
-- database cannot have the tables from 000027 through `make migrate`. What
-- dev, staging and production actually have has not been checked against
-- their catalogs: the foreign key rests on that header and on 0044, and the
-- second query under "Checking" settles it for a database. The real
-- constraint's delete action is not recorded; CASCADE is used here, as for
-- knowledge_base_id (user_id is NOT NULL, so SET NULL is impossible).
-- group_id has no foreign key: groups are not rows here (group_name is kept
-- beside the id), as in 000027.
--
-- Fresh databases get the tables with the foreign key. Where 000027 made the
-- tables, every CREATE below is a no-op (000027 made the same three indexes),
-- and nothing is dropped, rewritten or deleted. Two DO blocks, each its own
-- transaction under `make migrate` (psql -f, autocommit), give an existing
-- knowledge_base_manager_user the foreign key:
--
-- 1. A foreign key from user_id to "user" that is already there, whatever
--    its name or delete action, is left as it is. If another constraint
--    already holds the name knowledge_base_manager_user_user_id_fkey, nothing
--    is added and a WARNING says so: user_id stays without a foreign key.
--    Otherwise (a database that ran 000027) the constraint is added NOT
--    VALID, so no row is read and every new or changed row is checked from
--    then on. The ADD takes SHARE ROW EXCLUSIVE on knowledge_base_manager_user
--    and on "user": it waits for every open transaction that has written to
--    either table, and every writer of either that comes after waits behind
--    it. The locks are held until this block commits, which is straight
--    after the add.
-- 2. If that constraint is still NOT VALID (added by block 1 now, or by an
--    earlier run that stopped before this one), the rows that name a user
--    that does not exist are counted. If there are any, the constraint stays
--    NOT VALID, the rows are kept as they are and a WARNING gives the count.
--    Otherwise the constraint is validated. This takes ACCESS SHARE on both
--    tables for the count, then SHARE UPDATE EXCLUSIVE on
--    knowledge_base_manager_user and ROW SHARE on "user" for the validation,
--    none of which blocks reads or writes.
--
-- Creating knowledge_base_manager_user where it is missing takes SHARE ROW
-- EXCLUSIVE on knowledge_base and "user" (its two foreign keys) for that one
-- statement, and creating knowledge_base_manager_group takes it on
-- knowledge_base; neither reads a row. An index is built only where it is
-- missing: on a table created just above, which is empty, or on one that
-- neither 000027 nor this file made, where the build holds that table's
-- writers (SHARE) while it reads the rows. lock_timeout is 5s for every
-- statement here. Under `make migrate` a lock not granted in time fails that
-- statement with 55P03 (lock_not_available), nothing it did is kept, `make
-- migrate` stops and the file is not recorded. Run it again once the tables
-- are quiet; a run that stopped in block 2 leaves block 1's constraint NOT
-- VALID, and the next run validates it. The compose runner
-- (scripts/csm-compose/migrate-and-seed.sh) applies the file with psql -1, as
-- one transaction: there a failure keeps nothing of the file, and block 1's
-- locks last until the end of the file. It only serves local stacks.
--
-- Checking (read only). The file is recorded and never runs again, so its
-- WARNINGs are the only signal it gives. Run, then or later:
--   SELECT conrelid::regclass, conname FROM pg_constraint
--    WHERE contype = 'f' AND NOT convalidated;
-- lists every foreign key left NOT VALID, and
--   SELECT conname, pg_get_constraintdef(oid) FROM pg_constraint
--    WHERE conrelid = 'knowledge_base_manager_user'::regclass AND contype = 'f';
-- lists the table's foreign keys: one from user_id to "user" should be among
-- them, beside the one from knowledge_base_id.
--
-- Finishing by hand:
-- * Rows that name no user: delete or correct them, e.g.
--     DELETE FROM knowledge_base_manager_user m
--      WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = m.user_id);
--   (what ON DELETE CASCADE would have done), then run
--     ALTER TABLE knowledge_base_manager_user VALIDATE CONSTRAINT knowledge_base_manager_user_user_id_fkey;
-- * A taken name: see what that constraint is for; if it can go elsewhere,
--   rename it (ALTER TABLE knowledge_base_manager_user RENAME CONSTRAINT
--   knowledge_base_manager_user_user_id_fkey TO <another name>), then add the
--   key as block 1 would:
--     ALTER TABLE knowledge_base_manager_user ADD CONSTRAINT knowledge_base_manager_user_user_id_fkey
--       FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE NOT VALID;
--   and validate it as above.
--
-- Needs knowledge_base (0044) and "user" (0002).

SET lock_timeout = '5s';

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

    IF EXISTS (SELECT 1 FROM pg_constraint
                WHERE conrelid = 'knowledge_base_manager_user'::regclass
                  AND conname = 'knowledge_base_manager_user_user_id_fkey') THEN
        RAISE WARNING 'knowledge_base_manager_user.user_id: constraint knowledge_base_manager_user_user_id_fkey already exists and is not a foreign key from this column to "user", so no foreign key was added for it (see 0251_knowledge_base_manager_tables.sql)';
        RETURN;
    END IF;

    ALTER TABLE knowledge_base_manager_user
        ADD CONSTRAINT knowledge_base_manager_user_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE NOT VALID;
END
$$;

-- Validate in a transaction of its own: a DO block is one transaction, so
-- doing it above would hold the SHARE ROW EXCLUSIVE locks the ADD took
-- through the count and the validation scan.
DO $$
DECLARE
    orphans BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint con
          JOIN pg_attribute att
            ON att.attrelid = con.conrelid AND att.attnum = ANY (con.conkey)
         WHERE con.conrelid = 'knowledge_base_manager_user'::regclass
           AND con.conname = 'knowledge_base_manager_user_user_id_fkey'
           AND con.contype = 'f'
           AND con.confrelid = '"user"'::regclass
           AND cardinality(con.conkey) = 1
           AND att.attname = 'user_id'
           AND NOT con.convalidated
    ) THEN
        RETURN;
    END IF;

    SELECT count(*) INTO orphans
      FROM knowledge_base_manager_user m
     WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = m.user_id);

    IF orphans > 0 THEN
        RAISE WARNING 'knowledge_base_manager_user.user_id: % row(s) match no row of "user", so knowledge_base_manager_user_user_id_fkey stays NOT VALID until they are deleted or corrected (see 0251_knowledge_base_manager_tables.sql)', orphans;
        RETURN;
    END IF;

    ALTER TABLE knowledge_base_manager_user
        VALIDATE CONSTRAINT knowledge_base_manager_user_user_id_fkey;
END
$$;

RESET lock_timeout;
