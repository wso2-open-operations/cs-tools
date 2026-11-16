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

-- Inline images of a rich-text DESCRIPTION (case, service request, ...) are
-- uploaded before the record exists, with the same comment_attachment rows
-- comments use (migration 0150). There is no work item yet, so such an image is
-- tied to the PROJECT instead:
--
--   project_id         new. Set on a "draft description image": comment_id and
--                      work_item_id NULL. It stays set after linking.
--   description_image  true once the image is linked to the work item whose
--                      description shows it (work_item_id set, comment_id NULL).
--                      Marks it so the 24h unlinked-image cleanup never touches it.
--
-- A draft has a project and nothing else, so comment_attachment_has_parent now
-- accepts a project as a parent too.
--
-- Locking: as in 0150 the foreign key and CHECK are added NOT VALID and then
-- validated, and lock_timeout makes a busy table fail the file instead of
-- queueing. Every statement is idempotent: run it again after a timeout.
SET lock_timeout = '5s';

ALTER TABLE comment_attachment ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE comment_attachment ADD COLUMN IF NOT EXISTS description_image BOOLEAN NOT NULL DEFAULT false;

DO $$ BEGIN
    ALTER TABLE comment_attachment
        ADD CONSTRAINT comment_attachment_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Replace the parent CHECK (NOT VALID first, validated below).
ALTER TABLE comment_attachment DROP CONSTRAINT IF EXISTS comment_attachment_has_parent;
DO $$ BEGIN
    ALTER TABLE comment_attachment
        ADD CONSTRAINT comment_attachment_has_parent
        CHECK (comment_id IS NOT NULL OR work_item_id IS NOT NULL OR project_id IS NOT NULL) NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE comment_attachment VALIDATE CONSTRAINT comment_attachment_project_id_fkey;
ALTER TABLE comment_attachment VALIDATE CONSTRAINT comment_attachment_has_parent;

-- The cleanup job's queue of abandoned drafts.
CREATE INDEX IF NOT EXISTS idx_comment_attachment_unlinked_drafts
    ON comment_attachment (created_on)
    WHERE comment_id IS NULL AND work_item_id IS NULL AND project_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_comment_attachment_project_id
    ON comment_attachment (project_id) WHERE project_id IS NOT NULL;
