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

-- case_attachment.case_id pointed at "case"(id), the extension row of a plain
-- case, although its value is a work item id: the policies on this table
-- already read project membership through work_item, and every case-like type
-- (engagement, service request, security report analysis) has a work item.
--
-- Two things were wrong with the narrower target:
--
--   * an attachment pinned its "case" row (NO ACTION), so a case that has one
--     could not be moved to another type: a type transfer replaces the "case"
--     row with the row of the new type, and that delete was refused. Moving a
--     case to Security Report Analysis needs an attachment, so every such
--     transfer was blocked;
--   * a service request, engagement or security report analysis has no "case"
--     row at all, so none of them could own an attachment.
--
-- The target is now work_item(id), still with no ON DELETE action: a work item
-- with attachments is still not deletable, as before (its "case" row used to
-- be, through the cascade from work_item).
--
-- The old key was also what kept an attachment off every other kind of work
-- item (an announcement, a change request, an incident...). That rule is now
-- made by the two attachment INSERTs themselves (case_repo.go,
-- caseLikeNonAnnouncementTypes): a case-like work item other than an
-- announcement, nothing else. Apply this migration AFTER the entity-service
-- build that carries them is deployed: the other way round, the old build has
-- no such check and the narrower key is already gone. The build is safe without
-- the migration (a transfer of a case with attachments is refused with a 409).
--
-- The constraint is found by what it references, not by its name: it is
-- case_attachment_case_id_fkey where the table was created by 0106 and
-- case_attachments_case_id_fkey where it was renamed from its old plural name.
-- Idempotent: nothing is dropped once the constraint targets work_item, and
-- the new one is added only when it is missing (NOT VALID, then validated: see
-- below).
--
-- Dropping the old key and adding the new one take their locks (ACCESS EXCLUSIVE
-- on case_attachment, SHARE ROW EXCLUSIVE on work_item) for a moment only: the new
-- key is added NOT VALID, so there is no scan, and it is validated afterwards in a
-- separate transaction under a lock that blocks nothing. The lock_timeout makes the
-- file give up (rerun it) instead of queueing writers behind itself if work_item is
-- busy.

SET lock_timeout = '5s';

DO $$
DECLARE
    attachment_table regclass := to_regclass('case_attachment');
    case_col         smallint;
    old_fk           RECORD;
BEGIN
    IF attachment_table IS NULL THEN
        RETURN;
    END IF;

    SELECT attnum INTO case_col
      FROM pg_attribute
     WHERE attrelid = attachment_table AND attname = 'case_id' AND NOT attisdropped;
    IF case_col IS NULL THEN
        RETURN;
    END IF;

    -- A foreign key on case_id that targets the "case" table.
    FOR old_fk IN
        SELECT conname
          FROM pg_constraint
         WHERE conrelid = attachment_table
           AND contype = 'f'
           AND confrelid = to_regclass('"case"')
           AND conkey = ARRAY[case_col]
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', attachment_table, old_fk.conname);
    END LOOP;

    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = attachment_table
           AND contype = 'f'
           AND confrelid = to_regclass('work_item')
           AND conkey = ARRAY[case_col]
    ) THEN
        -- NOT VALID: the constraint is enforced for every new or changed row without
        -- scanning the existing ones, so this transaction (which also holds the
        -- locks the DROP above took) stays short, and a row the old constraint never
        -- checked (a database whose table was created without it) cannot abort this
        -- file and, with it, the migrations after it. It is validated below.
        ALTER TABLE case_attachment
            ADD CONSTRAINT case_attachment_work_item_id_fkey
            FOREIGN KEY (case_id) REFERENCES work_item(id) NOT VALID;
    END IF;
END $$;

-- Validate what is already there in a transaction of its own: a DO block is one
-- transaction, so doing it above would scan the table while still holding the
-- locks the DROP and the ADD took. On its own it needs only SHARE UPDATE
-- EXCLUSIVE, which does not block reads or writes. A row that points at no work
-- item is reported, not fatal, and the constraint stays NOT VALID (still enforced
-- for new rows); the next run of this file tries again.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = to_regclass('case_attachment')
           AND conname = 'case_attachment_work_item_id_fkey'
           AND NOT convalidated
    ) THEN
        BEGIN
            ALTER TABLE case_attachment VALIDATE CONSTRAINT case_attachment_work_item_id_fkey;
        EXCEPTION WHEN foreign_key_violation THEN
            RAISE NOTICE 'case_attachment has rows whose case_id is not a work item; the constraint is in place for new rows but left NOT VALID until they are fixed';
        END;
    END IF;
END $$;

RESET lock_timeout;
