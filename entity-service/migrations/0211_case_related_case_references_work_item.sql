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

-- "case".related_case_id pointed at "case"(id): a case's looser, non-hierarchical
-- link to another case. A ticket that is converted to another type (a query or an
-- incident found to be a migration becomes an engagement) stops being a "case" row
-- but is still the same ticket, and the tickets that name it as their related case
-- must stay related to it and stay what they are. With the old target that was
-- impossible: replacing the converted ticket's "case" row cleared the link on every
-- ticket pointing at it (ON DELETE SET NULL), and ServiceNow, which keeps the link,
-- could no longer sync it back.
--
-- The target is now work_item(id), still ON DELETE SET NULL, so the link follows
-- the ticket whatever type it becomes. The link is still a column of "case" (an
-- engagement, service request or security report analysis has none), so the
-- converted ticket's OWN outgoing link is not carried to its new type.
--
-- The constraint is found by what it references, not by its name. Idempotent:
-- nothing is dropped once the column targets work_item, and the new key is added
-- only when it is missing, NOT VALID first (no scan, so a stale row cannot abort the
-- file and the migrations after it) and validated afterwards in a transaction of its
-- own, under a lock that blocks nothing. A row that points at no work item is
-- reported, not fatal. related_case_id is
-- already indexed (idx_case_related_case_id), so checking it when a work item is
-- deleted stays an index lookup.
--
-- Apply this AFTER the entity-service build that reads the link through work_item
-- (the old build reads it through "case" and would not show a link to a converted
-- ticket). The build is safe without the migration: a conversion of a ticket that
-- others relate to is refused with a 409 until it is applied.

SET lock_timeout = '5s';

DO $$
DECLARE
    case_table regclass := to_regclass('"case"');
    related_col smallint;
    old_fk RECORD;
BEGIN
    IF case_table IS NULL THEN
        RETURN;
    END IF;

    SELECT attnum INTO related_col
      FROM pg_attribute
     WHERE attrelid = case_table AND attname = 'related_case_id' AND NOT attisdropped;
    IF related_col IS NULL THEN
        RETURN;
    END IF;

    -- A foreign key on related_case_id that targets "case" itself.
    FOR old_fk IN
        SELECT conname
          FROM pg_constraint
         WHERE conrelid = case_table
           AND contype = 'f'
           AND confrelid = case_table
           AND conkey = ARRAY[related_col]
    LOOP
        EXECUTE format('ALTER TABLE "case" DROP CONSTRAINT %I', old_fk.conname);
    END LOOP;

    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = case_table
           AND contype = 'f'
           AND confrelid = to_regclass('work_item')
           AND conkey = ARRAY[related_col]
    ) THEN
        ALTER TABLE "case"
            ADD CONSTRAINT case_related_case_id_work_item_fkey
            FOREIGN KEY (related_case_id) REFERENCES work_item(id) ON DELETE SET NULL NOT VALID;
    END IF;
END $$;

-- Validate what is already there in a transaction of its own: a DO block is one
-- transaction, so doing it above would scan "case" while still holding the locks
-- the DROP and the ADD took. On its own it needs only SHARE UPDATE EXCLUSIVE, which
-- does not block reads or writes. A row that points at no work item is reported,
-- not fatal, and the constraint stays NOT VALID (still enforced for new rows); the
-- next run of this file tries again.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = to_regclass('"case"')
           AND conname = 'case_related_case_id_work_item_fkey'
           AND NOT convalidated
    ) THEN
        BEGIN
            ALTER TABLE "case" VALIDATE CONSTRAINT case_related_case_id_work_item_fkey;
        EXCEPTION WHEN foreign_key_violation THEN
            RAISE NOTICE 'case has rows whose related_case_id is not a work item; the constraint is in place for new rows but left NOT VALID until they are fixed';
        END;
    END IF;
END $$;

RESET lock_timeout;
