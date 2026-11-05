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

-- A standard change template's history (migration 0277) is internal, like the
-- template itself (100026): only WSO2 staff read or write it.
--
-- DROP POLICY IF EXISTS before CREATE POLICY: safe to re-run.
BEGIN;

ALTER TABLE standard_change_template_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE standard_change_template_event FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS standard_change_template_event_internal_only ON standard_change_template_event;
CREATE POLICY standard_change_template_event_internal_only ON standard_change_template_event FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

COMMIT;
