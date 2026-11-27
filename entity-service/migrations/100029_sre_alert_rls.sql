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

-- Row-level security on the SRE alert table (migration 0308): internal only. The alert consumer, relay and processor
-- run under the system identity; nobody else reads or writes alert rows. Safe to run again.
SET lock_timeout = '5s';
BEGIN;
ALTER TABLE alert ENABLE ROW LEVEL SECURITY;
ALTER TABLE alert FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alert_internal_only ON alert;
CREATE POLICY alert_internal_only ON alert FOR ALL
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));
COMMIT;
