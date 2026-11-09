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

-- comment_attachment policies for inline images of a rich-text description
-- (migration 0250). Replaces the two policies of 100026 (not edited: it is
-- applied); UPDATE and DELETE stay internal-only.
--
-- SELECT, besides what 100026 allowed:
--   a DRAFT (comment_id and work_item_id NULL, project_id set) is visible to
--     its uploader while they are a member of the project;
--   a LINKED description image (comment_id NULL, description_image, work_item_id
--     set) is visible to anyone who can see the work item's project, so the
--     reader of a case sees the images in its description. NOT for a change
--     request: who may see one is decided by the customer-visibility rule
--     (CR_STRICT_VISIBILITY_FROM, designation), which lives in Go SQL and cannot
--     be restated here, so a project member would see the image of a change
--     request that was never designated to them. Its description images are
--     therefore internal-only (the work item's type is read under the caller's
--     own row-level security, so an invisible work item has no type and matches
--     nothing either).
-- INSERT: an external caller may also start a draft: PENDING, no storage_key,
--   project_id set, work_item_id NULL, theirs (created_by) and a project they are
--   a member of.
--
-- Idempotent (policies dropped and recreated); lock_timeout as in 100026.
SET lock_timeout = '5s';

BEGIN;

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
      AND work_item_id IS NOT NULL
      AND description_image = false
      AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
      AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = comment_attachment.work_item_id))
    )
    OR (
      comment_id IS NULL
      AND work_item_id IS NULL
      AND project_id IS NOT NULL
      AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
      AND is_project_member(comment_attachment.project_id)
    )
    OR (
      comment_id IS NULL
      AND description_image = true
      AND work_item_id IS NOT NULL
      AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = comment_attachment.work_item_id))
      AND (SELECT wi.type::TEXT FROM work_item wi WHERE wi.id = comment_attachment.work_item_id) <> 'CHANGE_REQUEST'
    )
  );

DROP POLICY IF EXISTS comment_attachment_write ON comment_attachment;
CREATE POLICY comment_attachment_write ON comment_attachment
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      status = 'PENDING'
      AND storage_key IS NULL
      AND comment_id IS NULL
      AND description_image = false
      AND lower(created_by) = lower(NULLIF((SELECT current_setting('app.viewer_email', true)), ''))
      AND (
        (work_item_id IS NOT NULL AND project_id IS NULL
          AND is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id)))
        OR
        (work_item_id IS NULL AND project_id IS NOT NULL
          AND is_project_member(project_id))
      )
    )
  );

COMMIT;
