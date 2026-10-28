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
-- them. That only happened where the plural products, users and cases tables
-- existed (000020 needs them), i.e. old local databases; databases the sync
-- built already have all three, and this migration does nothing there.
--
-- For each column: a foreign key from it to the right table that is already
-- there, whatever its name or delete action, is left as it is (and so is a
-- column whose constraint name 0044 uses is taken by something else, with a
-- NOTICE). Otherwise the
-- constraint is added under 0044's name, NOT VALID, so every new or changed
-- row is checked, and then validated. If existing rows point at a user or
-- work item that does not exist, validation is skipped with a NOTICE and the
-- rows are kept as they are: nothing is nulled, deleted or rewritten. To
-- finish by hand, clear or correct those values, e.g.
--   UPDATE knowledge_article a SET source_case_id = NULL
--    WHERE source_case_id IS NOT NULL
--      AND NOT EXISTS (SELECT 1 FROM work_item w WHERE w.id = a.source_case_id);
-- (which is what ON DELETE SET NULL would have done), then run
--   ALTER TABLE knowledge_article VALIDATE CONSTRAINT knowledge_article_source_case_id_fkey;
--
-- Adding a constraint takes a brief SHARE ROW EXCLUSIVE lock on
-- knowledge_article and on the table it references. work_item is row-level
-- secured, so the orphan count runs as an internal session (transaction-local
-- app.is_internal, as 0193 does); a count that still sees too few rows only
-- leaves a constraint NOT VALID, it cannot make validation fail.
--
-- Numbered after csm-sync-service's own 0261, since knowledge_article is its
-- table. Needs knowledge_article (0044), "user" (0002) and work_item (0021).

DO $$
DECLARE
    fk RECORD;
    orphans BIGINT;
BEGIN
    PERFORM set_config('app.is_internal', 'true', true);

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
            RAISE NOTICE 'knowledge_article: a constraint named % already exists and is not a foreign key from % to %; nothing added',
                fk.conname, fk.column_name, fk.target;
            CONTINUE;
        END IF;

        EXECUTE format(
            'ALTER TABLE knowledge_article ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %s(id) ON DELETE SET NULL NOT VALID',
            fk.conname, fk.column_name, fk.target);

        EXECUTE format(
            'SELECT count(*) FROM knowledge_article a WHERE a.%1$I IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %2$s t WHERE t.id = a.%1$I)',
            fk.column_name, fk.target)
           INTO orphans;

        IF orphans > 0 THEN
            RAISE NOTICE 'knowledge_article.%: % row(s) name a row that does not exist in %; % stays NOT VALID until they are cleared or corrected',
                fk.column_name, orphans, fk.target, fk.conname;
            CONTINUE;
        END IF;

        EXECUTE format('ALTER TABLE knowledge_article VALIDATE CONSTRAINT %I', fk.conname);
    END LOOP;
END
$$;
