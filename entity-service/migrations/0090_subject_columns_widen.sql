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

-- work_item.subject is the single shared column every work-item-type mapping
-- (incident, problem, change_request, change_task, incident_task,
-- incident_alert_task, problem_task, case, conversation) writes its
-- short_description/u_initial_message into - there's no separate subject
-- column per type to widen.
ALTER TABLE work_item ALTER COLUMN subject TYPE VARCHAR(512);

-- customer_engagement_status_update.subject is unrelated to work_item but
-- shares the same column name and needs the same widening.
ALTER TABLE customer_engagement_status_update ALTER COLUMN subject TYPE VARCHAR(512);
