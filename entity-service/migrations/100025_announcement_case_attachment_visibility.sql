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

-- Announcement visibility for an announcement's attachments.
--
-- An announcement can now own case attachments (a PDF sent with it). Migration
-- 100016 deliberately left case_attachment out of the announcement child-table
-- policies because "an announcement can never own an attachment"; that stopped
-- being true when announcement requests gained files, so the same rule 100016
-- gave comment and work_item_watcher is applied here: a project member who is
-- NOT cleared for a particular announcement (for example a General Access
-- contact and a security announcement, migration 100010) must not read its
-- attachments, or add to them, by case id -- the announcement row is hidden
-- from them, so everything hanging off it must be too.
--
-- Written exactly like 100016: each policy keeps its internal-or-project-member
-- rule and, for an ANNOUNCEMENT work item only, additionally requires the
-- announcement row itself to be visible to the caller (invoker rights, so the
-- read goes through announcement_visibility). One correlated work_item lookup,
-- the announcement check reached only when the row really is an ANNOUNCEMENT,
-- inlined by hand rather than wrapped in a helper function (100016 measured the
-- helper roughly 20x slower at the database). Internal callers short-circuit on
-- the first branch and pay nothing. For every other work item type the new
-- predicate is `wi.type <> 'ANNOUNCEMENT'` and changes nothing.
--
-- The internal-caller test keeps the InitPlan form 100015 introduced.
--
-- ALTER POLICY replaces a policy's expressions in place, so this is safe to
-- re-run. One transaction, for the reason 100016 gives: a failure partway must
-- not leave some policies on the new rule and the rest on the old one.
BEGIN;

ALTER POLICY case_attachment_visibility ON case_attachment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = case_attachment.case_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY case_attachment_write ON case_attachment
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = case_attachment.case_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY case_attachment_update ON case_attachment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = case_attachment.case_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  )
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = case_attachment.case_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

ALTER POLICY case_attachment_delete ON case_attachment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1 FROM work_item wi
      WHERE wi.id = case_attachment.case_id
        AND is_project_member(wi.project_id)
        AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
    )
  );

COMMIT;
