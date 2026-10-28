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

-- knowledge_article: the three foreign keys 0044 declares and the removed
-- 000027_replace_kb_tables_with_real_schema.up.sql left out.
--
-- 0044 (mirrored from csm-sync-service) creates knowledge_article with
--   author_id      REFERENCES "user"(id)    ON DELETE SET NULL
--   revised_by_id  REFERENCES "user"(id)    ON DELETE SET NULL
--   source_case_id REFERENCES work_item(id) ON DELETE SET NULL
-- 000027 created the same table with those three as plain UUIDs (its local
-- database had a plural users table and no work_item). 0044 is CREATE TABLE
-- IF NOT EXISTS, so a database where 000027 ran first kept the table without
-- them. That needs the plural products, users and cases tables 000020
-- references, which only old local databases had. Where 0044 made the table,
-- all three keys are there and this migration changes nothing.
--
-- That dev, staging and production got the table from 0044, and so already
-- have the three keys, has not been checked against their catalogs. It rests
-- on 000027's own header ("the real tables have FKs on
-- author_id/revised_by_id/user_id -> "user"(id) and source_case_id ->
-- work_item(id)"; git show
-- 67f35215a:entity-service/migrations/000027_replace_kb_tables_with_real_schema.up.sql)
-- and on csm-sync-service's 0044. The second query under "Checking" settles
-- it for a database.
--
-- Two statements, each its own transaction under `make migrate` (psql -f,
-- autocommit):
--
-- 1. For each column: a foreign key from it to the right table that is
--    already there, whatever its name or delete action, is left as it is. If
--    another constraint already holds 0044's name for it, nothing is added
--    and a WARNING says so: that column stays without a foreign key.
--    Otherwise the constraint is added under 0044's name, NOT VALID, so no
--    row is read and every new or changed row is checked from then on. Each
--    ADD takes SHARE ROW EXCLUSIVE on knowledge_article and on the table it
--    references ("user" or work_item): it waits for every open transaction
--    that has written to either table, and every writer of either that comes
--    after waits behind it. The locks are held until this statement commits,
--    which is straight after the adds.
-- 2. For each constraint under 0044's name that is still NOT VALID (added by
--    statement 1 now, or by an earlier run that stopped before this one):
--    the rows that point at a user or work item that does not exist are
--    counted. If there are any, the constraint stays NOT VALID, the rows are
--    kept as they are (nothing is nulled, deleted or rewritten) and a WARNING
--    gives the count. Otherwise the constraint is validated. This takes
--    ACCESS SHARE on both tables for the count, then SHARE UPDATE EXCLUSIVE
--    on knowledge_article and ROW SHARE on the referenced table for the
--    validation, none of which blocks reads or writes. work_item has
--    row-level policies, so the count runs as an internal session: the DO
--    block sets app.is_internal for its own transaction (0193 sets the same
--    flag for its whole session). knowledge_article has none, so the count
--    sees every article, and a work item it could not see would only leave a
--    constraint NOT VALID, never fail the validation.
--
-- lock_timeout is 5s for both. Under `make migrate` a lock not granted in
-- time fails that statement with 55P03 (lock_not_available), nothing it did
-- is kept, `make migrate` stops and the file is not recorded. Run it again
-- once the tables are quiet; a run that stopped in statement 2 leaves
-- statement 1's constraints NOT VALID, and the next run validates them. The
-- compose runner (scripts/csm-compose/migrate-and-seed.sh) applies the file
-- with psql -1, as one transaction: there a failure keeps nothing of the
-- file, and statement 1's locks last until the end of the file. It only
-- serves local stacks.
--
-- Checking (read only). The file is recorded and never runs again, so its
-- WARNINGs are the only signal it gives. Run, then or later:
--   SELECT conrelid::regclass, conname FROM pg_constraint
--    WHERE contype = 'f' AND NOT convalidated;
-- lists every foreign key left NOT VALID, and
--   SELECT conname, pg_get_constraintdef(oid) FROM pg_constraint
--    WHERE conrelid = 'knowledge_article'::regclass AND contype = 'f';
-- lists knowledge_article's foreign keys: author_id, revised_by_id and
-- source_case_id should each have one.
--
-- Finishing by hand:
-- * Rows that point nowhere: clear or correct those values, e.g.
--     UPDATE knowledge_article a SET source_case_id = NULL
--      WHERE source_case_id IS NOT NULL
--        AND NOT EXISTS (SELECT 1 FROM work_item w WHERE w.id = a.source_case_id);
--   (what ON DELETE SET NULL would have done), then run
--     ALTER TABLE knowledge_article VALIDATE CONSTRAINT knowledge_article_source_case_id_fkey;
--   Run that UPDATE as a superuser, or after
--   SELECT set_config('app.is_internal', 'true', false): otherwise work_item's
--   row-level policies hide every work item from it and it clears every
--   source_case_id. author_id and revised_by_id the same way, against "user".
-- * A taken name: see what that constraint is for; if it can go elsewhere,
--   rename it (ALTER TABLE knowledge_article RENAME CONSTRAINT
--   knowledge_article_author_id_fkey TO <another name>), then add the key as
--   statement 1 would, e.g.
--     ALTER TABLE knowledge_article ADD CONSTRAINT knowledge_article_author_id_fkey
--       FOREIGN KEY (author_id) REFERENCES "user"(id) ON DELETE SET NULL NOT VALID;
--   and validate it as above.
--
-- Numbered after csm-sync-service's own 0261, since knowledge_article is its
-- table. Needs knowledge_article (0044), "user" (0002) and work_item (0021).

SET lock_timeout = '5s';

DO $$
DECLARE
    fk RECORD;
BEGIN
    FOR fk IN
        SELECT column_name, target, conname
          FROM (VALUES
                ('author_id',      '"user"'::regclass,    'knowledge_article_author_id_fkey'),
                ('revised_by_id',  '"user"'::regclass,    'knowledge_article_revised_by_id_fkey'),
                ('source_case_id', 'work_item'::regclass, 'knowledge_article_source_case_id_fkey')
          ) AS wanted (column_name, target, conname)
    LOOP
        CONTINUE WHEN EXISTS (
            SELECT 1
              FROM pg_constraint con
              JOIN pg_attribute att
                ON att.attrelid = con.conrelid AND att.attnum = ANY (con.conkey)
             WHERE con.conrelid = 'knowledge_article'::regclass
               AND con.contype = 'f'
               AND con.confrelid = fk.target
               AND cardinality(con.conkey) = 1
               AND att.attname = fk.column_name
        );

        IF EXISTS (SELECT 1 FROM pg_constraint
                    WHERE conrelid = 'knowledge_article'::regclass AND conname = fk.conname) THEN
            RAISE WARNING 'knowledge_article.%: constraint % already exists and is not a foreign key from this column to %, so no foreign key was added for it (see 0262_knowledge_article_user_and_case_fks.sql)',
                fk.column_name, fk.conname, fk.target;
            CONTINUE;
        END IF;

        EXECUTE format(
            'ALTER TABLE knowledge_article ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %s(id) ON DELETE SET NULL NOT VALID',
            fk.conname, fk.column_name, fk.target);
    END LOOP;
END
$$;

-- Validate in a transaction of its own: a DO block is one transaction, so
-- doing it above would hold the SHARE ROW EXCLUSIVE locks the ADDs took
-- through the count and the validation scan.
DO $$
DECLARE
    fk RECORD;
    orphans BIGINT;
BEGIN
    PERFORM set_config('app.is_internal', 'true', true);

    FOR fk IN
        SELECT wanted.column_name, wanted.target, wanted.conname
          FROM (VALUES
                ('author_id',      '"user"'::regclass,    'knowledge_article_author_id_fkey'),
                ('revised_by_id',  '"user"'::regclass,    'knowledge_article_revised_by_id_fkey'),
                ('source_case_id', 'work_item'::regclass, 'knowledge_article_source_case_id_fkey')
          ) AS wanted (column_name, target, conname)
          JOIN pg_constraint con
            ON con.conrelid = 'knowledge_article'::regclass AND con.conname = wanted.conname
          JOIN pg_attribute att
            ON att.attrelid = con.conrelid AND att.attnum = ANY (con.conkey)
         WHERE con.contype = 'f'
           AND con.confrelid = wanted.target
           AND cardinality(con.conkey) = 1
           AND att.attname = wanted.column_name
           AND NOT con.convalidated
    LOOP
        EXECUTE format(
            'SELECT count(*) FROM knowledge_article a WHERE a.%1$I IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %2$s t WHERE t.id = a.%1$I)',
            fk.column_name, fk.target)
           INTO orphans;

        IF orphans > 0 THEN
            RAISE WARNING 'knowledge_article.%: % row(s) match no row of %, so % stays NOT VALID until they are cleared or corrected (see 0262_knowledge_article_user_and_case_fks.sql)',
                fk.column_name, orphans, fk.target, fk.conname;
            CONTINUE;
        END IF;

        EXECUTE format('ALTER TABLE knowledge_article VALIDATE CONSTRAINT %I', fk.conname);
    END LOOP;
END
$$;

RESET lock_timeout;
