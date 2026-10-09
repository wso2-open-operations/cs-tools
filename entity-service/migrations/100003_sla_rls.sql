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

-- Enforces project-membership visibility for `sla` at the database layer,
-- second table after case_escalation (migration 100002) to move off
-- Go-side project filtering.
--
-- Unlike case_escalation, `sla` has TWO real writers, both of which need to
-- keep working: the CSM-native SLA engine recompute worker (a background
-- process, always is_internal=true -- see NewSLAEngineRepository's own doc
-- comment) AND a customer-triggered write path: SLAEngineService.
-- ReviseCaseClocks runs synchronously on the SAME request context as
-- whatever caller just changed a case's severity (see
-- snCaseService.reviseCaseSLAClocks -- context.WithTimeout, not
-- WithoutCancel, so it is NOT detached and carries the real caller's
-- identity), and that caller is routinely an external customer updating
-- their own case. So unlike case_escalation's internal-only write policy,
-- every write policy here mirrors the SELECT policy's condition exactly:
-- is_internal OR is_project_member -- a customer's own severity change
-- must keep working, only a write against a project they don't belong to
-- should be rejected.
--
-- sla.work_item_id is nullable and, for INCIDENT/INCIDENT_TASK work items,
-- always NULL project_id (confirmed live against real data earlier in
-- this project) -- is_project_member(NULL) is simply false (no row can
-- match a NULL project_id), so those rows are correctly invisible to every
-- external caller and visible only to is_internal, with no separate
-- work_item-type branch needed.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE sla ENABLE ROW LEVEL SECURITY;
ALTER TABLE sla FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS sla_visibility ON sla;
CREATE POLICY sla_visibility ON sla
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = sla.work_item_id))
  );

DROP POLICY IF EXISTS sla_write ON sla;
CREATE POLICY sla_write ON sla
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
DROP POLICY IF EXISTS sla_update ON sla;
CREATE POLICY sla_update ON sla
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = sla.work_item_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
-- No production code path deletes an sla row (rows are transitioned to
-- CANCELLED/ACHIEVED/BREACHED, never removed) -- but internal/admin tooling
-- and test fixture cleanup legitimately do, so this is internal-only rather
-- than omitted entirely: an external caller was never going to delete an
-- sla row anyway, and FORCE + zero policy would have also blocked the one
-- legitimate internal case.
DROP POLICY IF EXISTS sla_delete ON sla;
CREATE POLICY sla_delete ON sla
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
