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

-- Removes row-level security from sla, incident, incident_task and problem
-- (added by migrations 100003 and 100009).
--
-- Why: none of the four is reachable by the customer portal. The customer
-- BFF (apps/customer-portal/backend-v2) never calls /slas, /incidents,
-- /problems or /incident-tasks; incident/incident_task/problem rows have no
-- project at all (work_item.project_id is NULL for all of them), and sla is
-- only read for customers through case queries whose work_item join is still
-- RLS-protected. The policies bought nothing for those tables but a planner
-- penalty: the "is_internal OR is_project_member(...)" qual is opaque to
-- cardinality estimation, which made /slas/search 15-23x slower for every
-- caller, internal ones included.
--
-- The protection these policies gave is replaced by an explicit
-- internal-caller-only gate on /slas/*, /incidents/*, /problems/* and
-- /incident-tasks/* (server/internal_only.go). That gate must ship together
-- with this migration: without it, any authenticated customer could read
-- these tables directly through the API.
--
-- Deliberately unchanged: work_item (still RLS-protected, so an incident or
-- case work_item is still invisible to non-members), announcement and
-- engagement (kept on purpose), and every other table in the series.
DO $$
DECLARE
  t TEXT;
  p RECORD;
BEGIN
  FOREACH t IN ARRAY ARRAY['sla', 'incident', 'incident_task', 'problem'] LOOP
    FOR p IN SELECT policyname FROM pg_policies WHERE schemaname = current_schema() AND tablename = t LOOP
      EXECUTE format('DROP POLICY %I ON %I', p.policyname, t);
    END LOOP;
    EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END
$$;
