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

ALTER TABLE work_item DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE change_request DROP COLUMN IF EXISTS customer_group_id;
ALTER TABLE service DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE service DROP COLUMN IF EXISTS support_group_id;
ALTER TABLE service DROP COLUMN IF EXISTS managed_by_group_id;
ALTER TABLE service DROP COLUMN IF EXISTS approval_group_id;
ALTER TABLE service DROP COLUMN IF EXISTS user_group_id;
ALTER TABLE communication_plan DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE account DROP COLUMN IF EXISTS cre_team_id;
ALTER TABLE account DROP COLUMN IF EXISTS sre_team_id;
ALTER TABLE project DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE incident_alert DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE catalog_item DROP COLUMN IF EXISTS group_id;
ALTER TABLE service_offering DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE service_offering DROP COLUMN IF EXISTS support_group_id;
ALTER TABLE service_offering DROP COLUMN IF EXISTS managed_by_group_id;
ALTER TABLE service_offering DROP COLUMN IF EXISTS approval_group_id;
ALTER TABLE service_offering DROP COLUMN IF EXISTS user_group_id;
ALTER TABLE customer_call DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE team_member DROP COLUMN IF EXISTS group_id;
ALTER TABLE deployment DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE deployment DROP COLUMN IF EXISTS support_group_id;
ALTER TABLE deployment DROP COLUMN IF EXISTS managed_by_group_id;
ALTER TABLE deployment DROP COLUMN IF EXISTS approval_group_id;
