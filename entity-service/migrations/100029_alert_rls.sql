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

-- alert is internal only: the Service Bus consumer writes it as the system identity, and no customer may read it.
BEGIN;

ALTER TABLE alert ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alert_internal_only ON alert;
CREATE POLICY alert_internal_only ON alert FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

COMMIT;
