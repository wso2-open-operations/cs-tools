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

-- The standard change suggestion snapshot (migration 0283) is internal, like the
-- dismissals (100029) and the catalogue (100026): only WSO2 staff and internal
-- services read or write it.
--
-- DROP POLICY IF EXISTS before CREATE POLICY: safe to re-run.
BEGIN;

ALTER TABLE standard_change_suggestion_run ENABLE ROW LEVEL SECURITY;
ALTER TABLE standard_change_suggestion_run FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS standard_change_suggestion_run_internal_only ON standard_change_suggestion_run;
CREATE POLICY standard_change_suggestion_run_internal_only ON standard_change_suggestion_run FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

ALTER TABLE standard_change_suggestion_run_customer ENABLE ROW LEVEL SECURITY;
ALTER TABLE standard_change_suggestion_run_customer FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS standard_change_suggestion_run_customer_internal_only ON standard_change_suggestion_run_customer;
CREATE POLICY standard_change_suggestion_run_customer_internal_only ON standard_change_suggestion_run_customer FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

ALTER TABLE standard_change_suggestion_snapshot ENABLE ROW LEVEL SECURITY;
ALTER TABLE standard_change_suggestion_snapshot FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS standard_change_suggestion_snapshot_internal_only ON standard_change_suggestion_snapshot;
CREATE POLICY standard_change_suggestion_snapshot_internal_only ON standard_change_suggestion_snapshot FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

COMMIT;
