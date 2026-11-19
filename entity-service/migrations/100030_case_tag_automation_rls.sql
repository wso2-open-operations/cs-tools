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

-- Row-level security for case_tag_automation (migration 0308). Internal only: the
-- shape of 100029_product_issue_rls.sql. The table names which tags the
-- system added to a case; customers never read or write it.
--
-- One transaction, every CREATE POLICY preceded by DROP POLICY IF EXISTS, as
-- elsewhere in this track.
BEGIN;

ALTER TABLE case_tag_automation ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_tag_automation FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS case_tag_automation_internal_only ON case_tag_automation;
CREATE POLICY case_tag_automation_internal_only ON case_tag_automation FOR ALL
    USING (current_setting('app.is_internal', true) = 'true')
    WITH CHECK (current_setting('app.is_internal', true) = 'true');

COMMIT;
