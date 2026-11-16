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

-- An inline image can be uploaded before the comment that shows it exists, so a
-- comment_attachment row must be able to exist without a comment. It is tied to
-- the work item instead: the file's storage path
-- (.../workitems/<work item>/inline_attachments/<number>_<name>) needs only the
-- work item, and so does the access check.
--
--   comment_id    now nullable: NULL until the comment is saved, then set to the
--                 comment whose content references the attachment.
--   work_item_id  new. Backfilled from comment.work_item_id for existing rows,
--                 and filled by a trigger whenever a row arrives with a comment
--                 but no work item, so csm-sync-service (whose mapping lists
--                 comment_id only) keeps inserting rows unchanged.
--
-- A row with neither is meaningless, so at least one must be set.
--
-- Locking: ADD CONSTRAINT ... FOREIGN KEY takes a lock on work_item that blocks
-- its writers while the whole table is checked (the shape that deadlocked a
-- running csm-sync-service in another RLS migration). The constraints are
-- therefore added NOT VALID and validated afterwards, which only takes a
-- lock that lets reads and writes continue. lock_timeout makes a busy table fail
-- the file instead of queueing behind a long query; every statement is
-- idempotent, so run it again after a timeout.
SET lock_timeout = '5s';

ALTER TABLE comment_attachment ADD COLUMN IF NOT EXISTS work_item_id UUID;

DO $$ BEGIN
    ALTER TABLE comment_attachment
        ADD CONSTRAINT comment_attachment_work_item_id_fkey
        FOREIGN KEY (work_item_id) REFERENCES work_item(id) ON DELETE CASCADE NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE OR REPLACE FUNCTION comment_attachment_fill_work_item() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF NEW.work_item_id IS NULL AND NEW.comment_id IS NOT NULL THEN
        SELECT c.work_item_id INTO NEW.work_item_id FROM comment c WHERE c.id = NEW.comment_id;
    END IF;
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS comment_attachment_fill_work_item ON comment_attachment;
CREATE TRIGGER comment_attachment_fill_work_item
    BEFORE INSERT OR UPDATE OF comment_id, work_item_id ON comment_attachment
    FOR EACH ROW EXECUTE FUNCTION comment_attachment_fill_work_item();

-- Rows that already exist.
UPDATE comment_attachment ca
SET work_item_id = c.work_item_id
FROM comment c
WHERE c.id = ca.comment_id AND ca.work_item_id IS NULL;

ALTER TABLE comment_attachment ALTER COLUMN comment_id DROP NOT NULL;

DO $$ BEGIN
    ALTER TABLE comment_attachment
        ADD CONSTRAINT comment_attachment_has_parent
        CHECK (comment_id IS NOT NULL OR work_item_id IS NOT NULL) NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE comment_attachment VALIDATE CONSTRAINT comment_attachment_work_item_id_fkey;
ALTER TABLE comment_attachment VALIDATE CONSTRAINT comment_attachment_has_parent;

CREATE INDEX IF NOT EXISTS idx_comment_attachment_work_item_id ON comment_attachment (work_item_id);

-- The cleanup job's work queue: images uploaded but never linked to a comment.
CREATE INDEX IF NOT EXISTS idx_comment_attachment_unlinked
    ON comment_attachment (created_on) WHERE comment_id IS NULL;
