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

-- Enforces project-membership visibility for case_escalation and
-- case_escalation_notification_list at the database layer, replacing what
-- was previously (and, on other branches, is still) a Go-side
-- authProjectIDs/scopePredicate filter built by hand in
-- EscalationRepository.SearchEscalations. That approach only protects a
-- call site that remembers to apply it; FORCE ROW LEVEL SECURITY protects
-- every statement against these two tables unconditionally, including ones
-- nobody remembers to gate.
--
-- This is the first table this mechanism is generalized to beyond
-- `announcement` (migration 100001) -- deliberately the simplest one
-- available: case_escalation has no customer-facing write path at all
-- (EscalationService.CreateEscalation always returns
-- ServiceUnavailableError -- see EscalationRepository's own doc comment),
-- so this migration only needs to reason about reads.
--
-- is_project_member(project_id) mirrors AccessRepository.RegisteredProjectIDs'
-- own SQL exactly (LOWER(email) match, state = 'REGISTERED') so the
-- membership rule can't drift between the two -- reused by every future
-- table's policy rather than copy-pasted, so a future change to what
-- "registered" means only has one place to change.
--
-- NULLIF on viewer_email, not a bare current_setting: current_setting(...,
-- true) returns '' (empty string), not NULL, once a transaction-local
-- set_config reverts at COMMIT/ROLLBACK and a later, unrelated statement on
-- the same pooled connection reads it before anything re-sets it. A bare ''
-- could in principle match a project_contact row with a blank email; NULLIF
-- turns that into NULL, which the membership EXISTS can never match,
-- keeping the fail-closed default intact.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS: CREATE POLICY has no IF NOT EXISTS form, so without this a
-- partial failure partway through this file (or a re-run against a database
-- where these policies were already applied by hand, outside
-- csm_migration_applied_migration) fails immediately on "policy already
-- exists" for whichever ones already landed, with no clean way to retry.
BEGIN;

CREATE OR REPLACE FUNCTION is_project_member(target_project_id UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
  SELECT EXISTS (
    SELECT 1
    FROM project_contact pc
    WHERE pc.project_id = target_project_id
      AND LOWER(pc.email) = LOWER(NULLIF(current_setting('app.viewer_email', true), ''))
      AND pc.state = 'REGISTERED'
  )
$$;

ALTER TABLE case_escalation ENABLE ROW LEVEL SECURITY;
-- Without FORCE, a non-superuser table owner is still exempt from its own
-- table's RLS policies -- see migration 100001's identical note. Confirmed
-- against this database's local role; see this branch's plan for the
-- explicit staging/production ownership check still outstanding before
-- this migration runs anywhere but local.
ALTER TABLE case_escalation FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_escalation_visibility ON case_escalation;
CREATE POLICY case_escalation_visibility ON case_escalation
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_escalation.work_item_id))
  );

-- No customer-facing writer exists today (see this file's own doc comment),
-- so writes are intentionally left to internal callers only, not
-- always-true like announcement's: FORCE with zero policies for a command
-- blocks that command entirely for every role, so an internal sync process
-- that DOES write this table needs is_internal=true set, not a bypass.
DROP POLICY IF EXISTS case_escalation_write_internal_only ON case_escalation;
CREATE POLICY case_escalation_write_internal_only ON case_escalation
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS case_escalation_update_internal_only ON case_escalation;
CREATE POLICY case_escalation_update_internal_only ON case_escalation
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS case_escalation_delete_internal_only ON case_escalation;
CREATE POLICY case_escalation_delete_internal_only ON case_escalation
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

-- case_escalation_notification_list has no project_id of its own -- it
-- reaches one via case_escalation.work_item_id, so its policy re-derives
-- the same membership check through that join rather than trusting the
-- parent row's own (already-RLS-protected) visibility implicitly.
ALTER TABLE case_escalation_notification_list ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_escalation_notification_list_visibility ON case_escalation_notification_list;
CREATE POLICY case_escalation_notification_list_visibility ON case_escalation_notification_list
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM case_escalation ce
      JOIN work_item wi ON wi.id = ce.work_item_id
      WHERE ce.id = case_escalation_notification_list.case_escalation_id
    ))
  );

DROP POLICY IF EXISTS case_escalation_notification_list_write_internal_only ON case_escalation_notification_list;
CREATE POLICY case_escalation_notification_list_write_internal_only ON case_escalation_notification_list
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS case_escalation_notification_list_update_internal_only ON case_escalation_notification_list;
CREATE POLICY case_escalation_notification_list_update_internal_only ON case_escalation_notification_list
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS case_escalation_notification_list_delete_internal_only ON case_escalation_notification_list;
CREATE POLICY case_escalation_notification_list_delete_internal_only ON case_escalation_notification_list
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
