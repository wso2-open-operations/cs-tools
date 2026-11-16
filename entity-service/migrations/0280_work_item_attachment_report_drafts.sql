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

-- A security report (SECURITY_REPORT_ANALYSIS) must have its report file before
-- the case exists. The file is uploaded first as a DRAFT work item attachment and
-- linked to the case in the same request that creates it (POST /cases with
-- attachmentIds), so a cancelled or refreshed upload leaves no case without a
-- report. As for a draft description image (migration 0250, comment_attachment) a
-- draft has no work item yet, so it hangs from the PROJECT:
--
--   project_id    new. Set on a draft: work_item_id is NULL until the case is
--                 created. It stays set after linking.
--   work_item_id  now nullable. Every list of a work item's files joins work_item,
--                 so a draft is never listed; it is visible only to its uploader
--                 (policy 100028).
--
-- work_item_attachment is also written by csm-sync-service, which always supplies
-- work_item_id, so relaxing the column changes nothing for it.
--
-- Locking: the foreign key and CHECK are added NOT VALID and then validated, and
-- lock_timeout makes a busy table fail the file instead of queueing. Every
-- statement is idempotent: run it again after a timeout.
SET lock_timeout = '5s';

ALTER TABLE work_item_attachment ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE work_item_attachment ALTER COLUMN work_item_id DROP NOT NULL;

DO $$ BEGIN
    ALTER TABLE work_item_attachment
        ADD CONSTRAINT work_item_attachment_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE work_item_attachment
        ADD CONSTRAINT work_item_attachment_has_parent
        CHECK (work_item_id IS NOT NULL OR project_id IS NOT NULL) NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE work_item_attachment VALIDATE CONSTRAINT work_item_attachment_project_id_fkey;
ALTER TABLE work_item_attachment VALIDATE CONSTRAINT work_item_attachment_has_parent;

-- The cleanup job's queue of drafts that never got a case.
CREATE INDEX IF NOT EXISTS idx_work_item_attachment_unlinked_drafts
    ON work_item_attachment (created_on)
    WHERE work_item_id IS NULL AND project_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_work_item_attachment_project_id
    ON work_item_attachment (project_id) WHERE project_id IS NOT NULL;
