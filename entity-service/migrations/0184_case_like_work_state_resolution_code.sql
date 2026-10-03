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

-- Work state and resolution code for service requests, engagements and
-- security report analyses.
--
-- ServiceNow keeps every case-like record in one table
-- (sn_customerservice_case), so u_work_state (1 = Ongoing, 2 = Paused) and
-- resolution_code exist on all of them, not just on "case". Checked on
-- wso2sndev: in-progress service requests, engagements and security report
-- analyses carry u_work_state (20 of 55, 11 of 17 and 4 of 6), and closed
-- ones carry resolution_code (11 of 44, 5 of 14 and 2 of 18). Announcements
-- carry neither, so they get no columns here.
--
-- Same enum types as "case" (0023), since the values come from the same
-- ServiceNow choice lists. Nullable with no default: a record that never
-- entered Work In Progress has no work state, as in ServiceNow.

ALTER TABLE service_request ADD COLUMN IF NOT EXISTS work_state case_work_state_enum;
ALTER TABLE service_request ADD COLUMN IF NOT EXISTS resolution_code case_resolution_code_enum;

ALTER TABLE engagement ADD COLUMN IF NOT EXISTS work_state case_work_state_enum;
ALTER TABLE engagement ADD COLUMN IF NOT EXISTS resolution_code case_resolution_code_enum;

ALTER TABLE security_report_analysis ADD COLUMN IF NOT EXISTS work_state case_work_state_enum;
ALTER TABLE security_report_analysis ADD COLUMN IF NOT EXISTS resolution_code case_resolution_code_enum;
