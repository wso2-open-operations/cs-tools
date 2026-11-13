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

-- Alert mute rules (migration 0278) are internal: only WSO2 staff and internal
-- services (alert-core, through csm-integration-service) read or write them.
-- Same shape as 100026: one FOR ALL policy on app.is_internal.
--
-- One transaction, every CREATE POLICY preceded by DROP POLICY IF EXISTS, as
-- elsewhere in this track.
BEGIN;

ALTER TABLE alert_mute_rule ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert_mute_rule FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alert_mute_rule_internal_only ON alert_mute_rule;
CREATE POLICY alert_mute_rule_internal_only ON alert_mute_rule FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

COMMIT;
