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


-- Row-level security for github_command (migration 0309). Internal only: readable
-- and writable by internal sessions (app.is_internal = 'true'), the shape of
-- 100029_product_issue_rls.sql. A product issue is itself invisible to customers,
-- and so is every command raised on it.
--
-- One transaction, DROP POLICY IF EXISTS before CREATE POLICY: safe to re-run.
BEGIN;

ALTER TABLE github_command ENABLE ROW LEVEL SECURITY;
ALTER TABLE github_command FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS github_command_internal_only ON github_command;
CREATE POLICY github_command_internal_only ON github_command FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

COMMIT;
