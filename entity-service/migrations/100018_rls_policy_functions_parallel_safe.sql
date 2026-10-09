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


-- One transaction, so the three labels change together or not at all
-- (`make migrate` runs each file with `psql -f`, no --single-transaction).
BEGIN;

-- Let Postgres use parallel plans for statements that touch an RLS-protected
-- table, by declaring the policies' helper functions PARALLEL SAFE.
--
-- Why: a user-defined function is PARALLEL UNSAFE unless declared otherwise,
-- and a single unsafe function anywhere in a policy makes every query that
-- reads that table non-parallel. That costs internal (staff) callers most:
-- they are unrestricted, so the policy filters nothing for them, yet they
-- lost the parallel sort-merge plans they get with RLS off. Measured on a
-- ~400K-row work_item copy, as an internal caller, with only this migration
-- applied (the SQL behind each endpoint): SLA search 233ms -> 28ms, incident
-- search 166ms -> 41ms; RLS off is ~30ms for both.
--
-- Why it is safe: each function is read-only (a STABLE SELECT) and depends
-- only on transaction-local settings (app.viewer_project_ids,
-- app.viewer_email, app.is_internal) and on tables read under the caller's
-- own RLS. A parallel worker restores the leader's settings and runs as the
-- same role, so it applies the same policy to the same rows. Checked with
-- forced 4-worker plans: customer row counts were identical to the serial
-- plan on work_item, comment, announcement, case and time_card, and a caller
-- with no settings still saw 0 rows everywhere.
--
-- The deployment and deployed_product policies (0176) use is_project_member
-- too, so they benefit from the same label.
--
-- ALTER (not CREATE OR REPLACE) on purpose: it changes only the parallel
-- label. Re-running an earlier migration that does CREATE OR REPLACE FUNCTION
-- on one of these (0149 defines announcement_is_security and
-- project_has_security_contact) without a PARALLEL clause silently resets it
-- to UNSAFE; re-apply this file afterwards.
ALTER FUNCTION is_project_member(UUID) PARALLEL SAFE;
ALTER FUNCTION announcement_is_security(UUID, announcement_type_enum) PARALLEL SAFE;
ALTER FUNCTION project_has_security_contact(UUID) PARALLEL SAFE;

COMMIT;
