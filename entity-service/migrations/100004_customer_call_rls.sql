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

-- Enforces project-membership visibility for customer_call (call requests)
-- at the database layer. Third table after case_escalation (0141) and
-- sla (0142). Unlike either of those, this was a previously ACKNOWLEDGED,
-- unfixed gap (call_request_repo.go's own prior doc comment: neither
-- SearchCallRequests nor SearchAllCallRequests ever did any caller-scoped
-- authorization at all) -- so this migration is closing a real, live
-- exposure, not hardening an already-partially-scoped table.
--
-- Both reads and writes need the SAME condition (is_internal OR
-- is_project_member): CreateCallRequest/UpdateCallRequest are customer-
-- facing (a customer opens/updates a call request on their own case), so
-- writes cannot be internal-only the way case_escalation's are.
--
-- customer_call.work_item_id is nullable (ON DELETE CASCADE, but a row can
-- in principle exist detached) -- is_project_member(NULL) is false, so a
-- detached row is correctly invisible to every external caller and visible
-- only to is_internal, consistent with every other nullable-join table in
-- this migration series.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE customer_call ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_call FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS customer_call_visibility ON customer_call;
CREATE POLICY customer_call_visibility ON customer_call
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = customer_call.work_item_id))
  );

DROP POLICY IF EXISTS customer_call_write ON customer_call;
CREATE POLICY customer_call_write ON customer_call
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
DROP POLICY IF EXISTS customer_call_update ON customer_call;
CREATE POLICY customer_call_update ON customer_call
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = customer_call.work_item_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
-- No DELETE policy: nothing in this codebase deletes a customer_call row.

COMMIT;
