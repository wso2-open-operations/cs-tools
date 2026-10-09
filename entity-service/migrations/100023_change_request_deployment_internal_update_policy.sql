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

-- An internal-only UPDATE policy on change_request_deployment, the junction csm-sync-service
-- owns (0136_change_request_deployment_table.sql: a surrogate id plus UNIQUE(change_request_id,
-- deployment_id), fanned out from change_request.u_deployments by expand_list).
--
-- Why: migration 100024 (originally inline in 0191, split out into the RLS track -- see that
-- file's own header) put FORCE ROW LEVEL SECURITY on that table with a SELECT, an INSERT and a
-- DELETE policy, and deliberately no UPDATE one ("the repository only ever deletes and re-inserts
-- the whole list"). That is true of entity-service, but the sync writes the table the way it writes
-- work_item_watcher: INSERT ... ON CONFLICT (change_request_id, deployment_id) DO UPDATE, and the
-- conflict branch is an UPDATE. With RLS forced and no UPDATE policy, that branch is refused for
-- every caller ("new row violates row-level security policy (USING expression)"), the same failure
-- migration 100021 fixed for seven other table/command pairs. The sync connects with
-- app.is_internal = true, so the policy below is the same internal-only shape as 100021's.
--
-- There is deliberately no project-member branch: a customer session is refused an UPDATE exactly
-- as before. No column, table or type changes; this adds one policy. This file runs AFTER 100024
-- in the RLS track (100023 < 100024), which is fine: CREATE POLICY has no dependency on another
-- policy, or on ENABLE/FORCE ROW LEVEL SECURITY, already having run against the same table -- by
-- the time every migration in this track has run, the end state is identical either way.
--
-- change_request_deployed_product (also from 100024) gets no UPDATE policy: it is ours, not the
-- sync's, and nothing writes it except entity-service's delete-and-re-insert. The pair is listed in
-- rlsCommandsDeniedOnPurpose (rls_schema_integration_test.go) with that reason.
--
-- Locking: CREATE POLICY and DROP POLICY take an ACCESS EXCLUSIVE lock on the table, so
-- lock_timeout makes a busy table fail the file instead of queueing behind a long query.
-- Idempotent (the policy is dropped and recreated), so after a lock timeout, run it again.
--
-- The internal check uses the planner-friendly scalar sub-select introduced by migration 100015.
SET lock_timeout = '5s';

DROP POLICY IF EXISTS change_request_deployment_update_internal_only ON change_request_deployment;
CREATE POLICY change_request_deployment_update_internal_only ON change_request_deployment
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

RESET lock_timeout;
