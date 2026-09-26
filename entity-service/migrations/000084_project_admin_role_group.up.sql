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

-- Admin becomes a PROJECT role.
--
-- Salesforce's `Admin` used to map straight to the GLOBAL customer_admin /
-- partner_admin role on the user, with nothing recorded per project. Under
-- the rewritten onboarding flow admin is stored per project and the
-- account-level role is derived from it: admin on any project under an
-- account means admin on every project under that account, and nothing
-- outside it.
--
-- project_role_enum ALREADY carries 'ADMIN' (migration 000023 declared all
-- five values up front), so there is no enum to widen -- what is missing is
-- the vocabulary a membership can actually be attached to. A membership
-- reaches its roles through project_contact_group -> project_group ->
-- project_group_role -> project_role, never through project_role directly,
-- so storing "admin on this project" needs a project_group whose role set is
-- {ADMIN}. That is what this migration seeds.
--
-- Idempotent, and deliberately so: project_group/project_role rows are
-- normally seeded by the ServiceNow sync (see the 503 the membership upsert
-- raises for a missing group), so this must be safe to run against a database
-- that already has them.

INSERT INTO project_role (id, created_on, updated_on, created_by, updated_by, role)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-000084', 'migration-000084', 'ADMIN'::project_role_enum
WHERE NOT EXISTS (SELECT 1 FROM project_role WHERE role = 'ADMIN'::project_role_enum);

INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group")
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-000084', 'migration-000084', 'Admin'
WHERE NOT EXISTS (SELECT 1 FROM project_group WHERE "group" = 'Admin');

INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-000084', 'migration-000084', pg.id, pr.id
FROM project_group pg, project_role pr
WHERE pg."group" = 'Admin'
  AND pr.role = 'ADMIN'::project_role_enum
  AND NOT EXISTS (
      SELECT 1 FROM project_group_role x
      WHERE x.project_group_id = pg.id AND x.project_role_id = pr.id
  );
