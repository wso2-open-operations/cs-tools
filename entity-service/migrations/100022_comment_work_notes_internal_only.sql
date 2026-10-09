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

-- Work notes are internal: keep them out of external sessions at the database,
-- in line with what the customer portal backend already does in code.
--
-- Comments of type WORK_NOTE are the support team's internal notes. The
-- customer portal backend removes them in Go (apps/customer-portal/backend-v2,
-- dto.MapSearchCaseActivities), but the comment policies from 100008/100016 only
-- ask for project membership, so the database itself did not distinguish them.
-- Any caller of entity-service that did not apply the Go filter, and the
-- activity total (counted before that filter), saw them too.
--
-- For an external caller, SELECT, INSERT and UPDATE of a comment row now also
-- require that the row is not a WORK_NOTE. Internal callers short-circuit on
-- the first branch and are unchanged. A NULL type stays visible (IS DISTINCT
-- FROM), exactly as the portal backend treats it today. The portal only ever
-- creates type COMMENT (backend-v2 dto/case.go), so the write side loses
-- nothing. APPROVAL_HISTORY stays visible: the portal shows it.
--
-- comment_edit_history needs no change: its policies reach the project through
-- a join on comment, which is itself filtered by comment_visibility, so the
-- edit history of a hidden work note resolves to no project and is hidden too.
-- comment_delete_internal_only is already internal-only.
--
-- ALTER POLICY replaces the expressions in place, so this is safe to re-run.
-- One transaction (see 100016): all three policies move together.
BEGIN;

ALTER POLICY comment_visibility ON comment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      comment.type IS DISTINCT FROM 'WORK_NOTE'
      AND EXISTS (
        SELECT 1 FROM work_item wi
        WHERE wi.id = comment.work_item_id
          AND is_project_member(wi.project_id)
          AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
      )
    )
  );

ALTER POLICY comment_write ON comment
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      comment.type IS DISTINCT FROM 'WORK_NOTE'
      AND EXISTS (
        SELECT 1 FROM work_item wi
        WHERE wi.id = comment.work_item_id
          AND is_project_member(wi.project_id)
          AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
      )
    )
  );

ALTER POLICY comment_update ON comment
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      comment.type IS DISTINCT FROM 'WORK_NOTE'
      AND EXISTS (
        SELECT 1 FROM work_item wi
        WHERE wi.id = comment.work_item_id
          AND is_project_member(wi.project_id)
          AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
      )
    )
  )
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      comment.type IS DISTINCT FROM 'WORK_NOTE'
      AND EXISTS (
        SELECT 1 FROM work_item wi
        WHERE wi.id = comment.work_item_id
          AND is_project_member(wi.project_id)
          AND (wi.type <> 'ANNOUNCEMENT' OR EXISTS (SELECT 1 FROM announcement a WHERE a.id = wi.id))
      )
    )
  );

COMMIT;
