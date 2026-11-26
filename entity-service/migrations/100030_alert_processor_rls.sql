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

-- alert_outbox and alert_group are internal only, like alert: the alert consumer and processor use them as the system identity.
ALTER TABLE alert_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert_outbox FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alert_outbox_internal_only ON alert_outbox;
CREATE POLICY alert_outbox_internal_only ON alert_outbox FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

ALTER TABLE alert_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert_group FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alert_group_internal_only ON alert_group;
CREATE POLICY alert_group_internal_only ON alert_group FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');
