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

-- Enforces the deny-all policy for incident/incident_task/problem: unlike
-- every other table in this series, these three have NO project concept at
-- all (confirmed live, this branch's own earlier session: 100% NULL
-- work_item.project_id for every INCIDENT/INCIDENT_TASK/PROBLEM row), so
-- their policy is simply is_internal, with no is_project_member OR-branch
-- -- an external caller sees and writes none of them, full stop. This
-- replaces the class of duplicated requireInternalCaller-style Go checks
-- these three services relied on with one DB-level rule.
--
-- The Go side (incident_repo.go/incident_task_repo.go/problem_repo.go) was
-- already converted to Scoped in the previous migration (0147), since
-- work_item's own new RLS affected them immediately regardless of whether
-- incident/incident_task/problem had their own policies yet -- this
-- migration adds no new Go-side identity plumbing, only the policies
-- themselves.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE incident ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS incident_deny_all_select ON incident;
CREATE POLICY incident_deny_all_select ON incident
  FOR SELECT USING (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS incident_deny_all_insert ON incident;
CREATE POLICY incident_deny_all_insert ON incident
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS incident_deny_all_update ON incident;
CREATE POLICY incident_deny_all_update ON incident
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS incident_deny_all_delete ON incident;
CREATE POLICY incident_deny_all_delete ON incident
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

ALTER TABLE incident_task ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_task FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS incident_task_deny_all_select ON incident_task;
CREATE POLICY incident_task_deny_all_select ON incident_task
  FOR SELECT USING (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS incident_task_deny_all_insert ON incident_task;
CREATE POLICY incident_task_deny_all_insert ON incident_task
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS incident_task_deny_all_update ON incident_task;
CREATE POLICY incident_task_deny_all_update ON incident_task
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS incident_task_deny_all_delete ON incident_task;
CREATE POLICY incident_task_deny_all_delete ON incident_task
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

ALTER TABLE problem ENABLE ROW LEVEL SECURITY;
ALTER TABLE problem FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS problem_deny_all_select ON problem;
CREATE POLICY problem_deny_all_select ON problem
  FOR SELECT USING (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS problem_deny_all_insert ON problem;
CREATE POLICY problem_deny_all_insert ON problem
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS problem_deny_all_update ON problem;
CREATE POLICY problem_deny_all_update ON problem
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS problem_deny_all_delete ON problem;
CREATE POLICY problem_deny_all_delete ON problem
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
