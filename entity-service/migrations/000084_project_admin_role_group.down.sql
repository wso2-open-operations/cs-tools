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

-- Only rows this migration itself inserted are removed. The ServiceNow sync
-- seeds the same vocabulary, and dropping its rows on a rollback would break
-- every membership upsert (a missing project_group is a 503 there).
-- project_contact_group rows referencing the Admin group cascade with it,
-- which is correct: without the group there is no way to express the role.
DELETE FROM project_group_role
WHERE created_by = 'migration-000084';

DELETE FROM project_group
WHERE "group" = 'Admin' AND created_by = 'migration-000084';

DELETE FROM project_role
WHERE role = 'ADMIN'::project_role_enum AND created_by = 'migration-000084';
