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

-- Announcement visibility for a hidden announcement's child rows.
--
-- Problem (found in review of #2094 and reproduced): comment and
-- work_item_watcher were gated on project membership alone (migration
-- 0147). A project member who is NOT cleared for a particular announcement
-- (for example a General Access contact and a security announcement,
-- migration 100010) could still read its comments, or add to them, by
-- work-item UUID: the announcement row was hidden but everything hanging off
-- it was not.
--
-- Fix: each policy keeps its internal-or-project-member rule and, for an
-- ANNOUNCEMENT work item only, additionally requires the announcement row
-- itself to be visible to the caller (invoker rights, so the read goes
-- through announcement_visibility). It is written as ONE correlated
-- work_item lookup (the same single probe the old rule already made), with
-- the announcement check reached only when the row really is an
-- ANNOUNCEMENT. An earlier draft wrapped this in a helper function; that
-- added a second RLS-checked work_item probe per row and made comment reads
-- roughly 20x slower at the database, so it was inlined by hand. Internal
-- callers short-circuit on the first branch and pay nothing.
--
-- case_attachment is deliberately NOT covered: its case_id references
-- "case", and an announcement has no "case" row, so an announcement can
-- never own an attachment. Adding the predicate there would be pure
-- overhead.
--
-- work_item_tag is deliberately NOT covered: announcement_is_security()
-- reads work_item_tag, so requiring announcement visibility there would make
-- announcement_visibility recurse into itself. A tag name on a hidden
-- announcement is far less sensitive than its comments and attachments.
--
-- ALTER POLICY replaces a policy's expressions in place, so this is safe to
-- re-run.

-- One transaction: `make migrate` runs each file with `psql -f` and no
-- --single-transaction, so a failure partway would leave the comment policies on
-- the new rule and the watcher policies on the old one (announcement children
-- visible again) until a retry. All or nothing instead.
BEGIN;

-- comment

ALTER POLICY comment_visibility ON comment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = comment.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY comment_write ON comment
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = comment.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY comment_update ON comment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = comment.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  )
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = comment.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

-- work_item_watcher

ALTER POLICY work_item_watcher_visibility ON work_item_watcher
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = work_item_watcher.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY work_item_watcher_write ON work_item_watcher
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = work_item_watcher.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY work_item_watcher_delete ON work_item_watcher
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = work_item_watcher.work_item_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

COMMIT;
