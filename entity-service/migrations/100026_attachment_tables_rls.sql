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

-- Project-membership visibility for work_item_attachment, comment_attachment and
-- deployment_attachment. Until now none of the three had row-level security, so
-- the database did not distinguish a customer from an internal user when
-- attachment metadata was read: that was done (or not) in Go.
--
-- An attachment is visible exactly when its parent is:
--   work_item_attachment   the work item's project, as for sla (migration 100003)
--   deployment_attachment  the deployment's project
--   comment_attachment     the comment, through comment's own policy, so an
--                          attachment of a WORK_NOTE stays internal (100022)
--                          without this file restating that rule. A row not yet
--                          linked to a comment (comment_id NULL: an inline image
--                          uploaded before its comment was saved, migration
--                          0150) is visible only to the person who uploaded it.
-- is_project_member is what decides membership, so the REGISTERED-only rule
-- stays in one place. A parent with no project (project_id NULL) matches no
-- member, so an external caller never sees such an attachment.
--
-- WRITES. An external caller may only INSERT, and only a row that has not been
-- stored yet: status PENDING and no storage_key, so a caller cannot register a
-- file at a path of their choosing. Confirming an upload (storage_key, status),
-- linking inline images to their comment, expiry and deletion are done by
-- entity-service itself, which opens an internal session after checking the
-- caller, so UPDATE and DELETE are internal-only (as in migration 100021).
-- csm-sync-service connects with app.is_internal = true and every policy has an
-- internal branch for all four commands, so its upserts and delete-sync keep
-- working. With FORCE ROW LEVEL SECURITY a command with no policy is refused
-- for everyone, internal sessions included, hence all four per table
-- (TestRLSSchemaIntegration_EveryProtectedTableHasAPolicyForEveryCommand).
--
-- Locking: CREATE/DROP POLICY take ACCESS EXCLUSIVE on the table. Each table is
-- its own short transaction and lock_timeout makes a busy table fail the file
-- instead of stalling everything behind it (see 100021). The file is
-- idempotent: each policy is dropped and recreated, so run it again after a
-- timeout.
SET lock_timeout = '5s';

-- ---------------------------------------------------------------------------
-- work_item_attachment
-- ---------------------------------------------------------------------------
BEGIN;

ALTER TABLE work_item_attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_attachment FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_attachment_visibility ON work_item_attachment;
CREATE POLICY work_item_attachment_visibility ON work_item_attachment
  FOR SELECT
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_attachment.work_item_id))
  );

DROP POLICY IF EXISTS work_item_attachment_write ON work_item_attachment;
CREATE POLICY work_item_attachment_write ON work_item_attachment
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      status = 'PENDING'
      AND storage_key IS NULL
      AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
    )
  );

DROP POLICY IF EXISTS work_item_attachment_update_internal_only ON work_item_attachment;
CREATE POLICY work_item_attachment_update_internal_only ON work_item_attachment
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS work_item_attachment_delete_internal_only ON work_item_attachment;
CREATE POLICY work_item_attachment_delete_internal_only ON work_item_attachment
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

COMMIT;

-- ---------------------------------------------------------------------------
-- comment_attachment
-- ---------------------------------------------------------------------------
BEGIN;

ALTER TABLE comment_attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE comment_attachment FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS comment_attachment_visibility ON comment_attachment;
CREATE POLICY comment_attachment_visibility ON comment_attachment
  FOR SELECT
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      comment_id IS NOT NULL
      AND EXISTS (SELECT 1 FROM comment c WHERE c.id = comment_attachment.comment_id)
    )
    OR (
      comment_id IS NULL
      AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
      AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = comment_attachment.work_item_id))
    )
  );

-- An external caller can only start an upload: not yet stored, not yet linked,
-- on a work item they belong to, and recorded as theirs.
DROP POLICY IF EXISTS comment_attachment_write ON comment_attachment;
CREATE POLICY comment_attachment_write ON comment_attachment
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      status = 'PENDING'
      AND storage_key IS NULL
      AND comment_id IS NULL
      AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
      AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
    )
  );

DROP POLICY IF EXISTS comment_attachment_update_internal_only ON comment_attachment;
CREATE POLICY comment_attachment_update_internal_only ON comment_attachment
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS comment_attachment_delete_internal_only ON comment_attachment;
CREATE POLICY comment_attachment_delete_internal_only ON comment_attachment
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

COMMIT;

-- ---------------------------------------------------------------------------
-- deployment_attachment
-- ---------------------------------------------------------------------------
BEGIN;

ALTER TABLE deployment_attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployment_attachment FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS deployment_attachment_visibility ON deployment_attachment;
CREATE POLICY deployment_attachment_visibility ON deployment_attachment
  FOR SELECT
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member((SELECT d.project_id FROM deployment d WHERE d.id = deployment_attachment.deployment_id))
  );

DROP POLICY IF EXISTS deployment_attachment_write ON deployment_attachment;
CREATE POLICY deployment_attachment_write ON deployment_attachment
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      status = 'PENDING'
      AND storage_key IS NULL
      AND is_project_member((SELECT d.project_id FROM deployment d WHERE d.id = deployment_id))
    )
  );

DROP POLICY IF EXISTS deployment_attachment_update_internal_only ON deployment_attachment;
CREATE POLICY deployment_attachment_update_internal_only ON deployment_attachment
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS deployment_attachment_delete_internal_only ON deployment_attachment;
CREATE POLICY deployment_attachment_delete_internal_only ON deployment_attachment
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

COMMIT;
