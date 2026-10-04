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

-- Internal notes (comment.type = 'WORK_NOTE') are for staff only.
--
-- Until now the comment policies (0147, last amended by 0175) granted every
-- row of a work item to any member of its project, whatever the row's type,
-- and no Postgres read path (case comment list, case activity feed, generic
-- comment search) filtered on type for a non-internal caller. So a
-- registered project contact read every work note on their cases, and could
-- write one.
--
-- The rule now lives in the policies, so every path that reads or writes
-- `comment` through a caller identity gets it, including ones added later:
--   - SELECT: a non-internal session sees a row only when it is not a
--     WORK_NOTE (COMMENT and APPROVAL_HISTORY rows are unchanged).
--   - INSERT: a non-internal session cannot create a WORK_NOTE.
--   - UPDATE: a non-internal session can neither edit a WORK_NOTE nor turn
--     a row into one.
-- Internal sessions (app.is_internal = 'true') are unaffected. DELETE is
-- already internal-only (comment_delete_internal_only).
--
-- `IS DISTINCT FROM` keeps a row with a NULL type (the column is nullable)
-- visible to members, as it was. The membership and announcement parts of
-- each expression are 0175's, unchanged.
--
-- ALTER POLICY replaces the expressions in place, so this is safe to re-run.
-- One transaction, so the three policies never disagree; lock_timeout so it
-- fails fast instead of queueing behind a long transaction on `comment`.

BEGIN;

SET LOCAL lock_timeout = '5s';

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
