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

-- work_item_attachment policies for a DRAFT report (migration 0280): uploaded to a
-- project before the case it belongs to exists. Replaces the SELECT and INSERT
-- policies of 100026 (not edited: it is applied); UPDATE and DELETE stay
-- internal-only.
--
-- SELECT, besides what 100026 allowed:
--   a DRAFT (work_item_id NULL, project_id set) is visible to its uploader while
--   they are a member of the project. Once linked it is visible like any other
--   attachment of its work item.
-- INSERT: an external caller may also start a draft: PENDING, no storage_key,
--   project_id set, work_item_id NULL, theirs (created_by) and a project they are a
--   member of.
--
-- Idempotent (policies dropped and recreated); lock_timeout as in 100026.
SET lock_timeout = '5s';

BEGIN;

DROP POLICY IF EXISTS work_item_attachment_visibility ON work_item_attachment;
CREATE POLICY work_item_attachment_visibility ON work_item_attachment
  FOR SELECT
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      work_item_id IS NOT NULL
      AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_attachment.work_item_id))
    )
    OR (
      work_item_id IS NULL
      AND project_id IS NOT NULL
      AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
      AND is_project_member(work_item_attachment.project_id)
    )
  );

DROP POLICY IF EXISTS work_item_attachment_write ON work_item_attachment;
CREATE POLICY work_item_attachment_write ON work_item_attachment
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      status = 'PENDING'
      AND storage_key IS NULL
      AND (
        (work_item_id IS NOT NULL AND project_id IS NULL
          AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id)))
        OR
        (work_item_id IS NULL AND project_id IS NOT NULL
          AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
          AND is_project_member(project_id))
      )
    )
  );

COMMIT;
