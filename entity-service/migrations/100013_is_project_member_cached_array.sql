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

-- Performance fix, no change in what is authorized: is_project_member's
-- body changes from a per-call EXISTS query against project_contact to a
-- literal-array membership check against app.viewer_project_ids -- a new
-- session-local GUC Scoped/rls.go now compute and cache once per
-- statement/transaction (scoped.go's queueIdentity, rls.go's
-- setCallerIdentity), alongside the existing app.is_internal/
-- app.viewer_email pair.
--
-- Why: confirmed empirically (EXPLAIN ANALYZE against a 20k-row synthetic
-- case load on one project, real data otherwise) that the EXISTS-per-call
-- form is invisible to the query planner as an indexable condition --
-- work_item has idx_work_item_project_id, but a policy that calls
-- is_project_member(project_id) as an opaque function forces a sequential
-- scan of the ENTIRE work_item table for every external caller's query,
-- regardless of how few rows their own projects actually have. That scan
-- cost grows with total system-wide case volume, not the caller's own
-- data, and got measurably worse as work_item grows -- 1.3s for a single
-- paginated case search in the synthetic test, entirely from this one
-- planner limitation.
--
-- This function's own SIGNATURE is unchanged (is_project_member(uuid) ->
-- boolean), so every existing policy across every prior migration in this
-- series keeps working with zero changes to their own text: Postgres
-- inlines a simple LANGUAGE SQL STABLE function like this one, so
-- is_project_member(project_id) in a policy becomes, after inlining,
-- textually equivalent to the array check below -- confirmed directly:
-- EXPLAIN on the unmodified caseLikeJoins query, after only this
-- redefinition, shows "project_id = ANY(...current_setting...)" in the
-- Filter line, not a function call, and the query runs measurably faster.
--
-- Deliberately NOT a Go-resolved project list: AccessService.ResolveScope
-- still only ever produces {Unrestricted, ViewerEmail} -- exactly what it
-- produced before this migration. The database, not Go, computes
-- app.viewer_project_ids, from viewerEmail alone, in the same
-- identity-setting step that already runs app.is_internal/
-- app.viewer_email. This is the load-bearing distinction from a
-- Go-side-resolved-and-forwarded project list (rejected earlier in this
-- series' review): Go's contract with the database is unchanged, and this
-- array can never be the sole source of truth for authorization -- every
-- policy still ORs it with is_internal and still runs underneath FORCE ROW
-- LEVEL SECURITY; a caller who somehow reached Postgres with this GUC
-- unset gets '{}' (via NULLIF) matching nothing, the same fail-closed
-- default as always.
--
-- NULLIF(..., '') guards the same edge case is_project_member's own
-- previous body already guarded: current_setting(..., true) returns '' (not
-- NULL) once a transaction-local set_config reverts and a later statement
-- on the same pooled connection reads it before anything re-sets it; NULLIF
-- turns that into NULL, and project_id = ANY(NULL::uuid[]) is NULL (falsy),
-- never true.
CREATE OR REPLACE FUNCTION is_project_member(target_project_id UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
  SELECT target_project_id = ANY(NULLIF(current_setting('app.viewer_project_ids', true), '')::uuid[])
$$;
