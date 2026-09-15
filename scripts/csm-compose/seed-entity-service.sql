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
--
-- Minimal seed data for the docker-compose local dev stack (see
-- apps/csm-portal/README.md). entity-service's own migrations create the
-- schema only -- no seed tooling exists upstream, so this file is applied
-- as a one-off step after migrations. All values are fake/dummy, safe for
-- a public repo (no real names, emails, or ids).
--
-- Idempotent: safe to run more than once (ON CONFLICT DO NOTHING throughout).

BEGIN;

-- Roles. Names matter here: recompute_user_type() (migration 000007) treats
-- 'admin'/'internal' as INTERNAL and 'customer'/'external'/... as EXTERNAL.
INSERT INTO role (id, created_on, updated_on, name, description) VALUES
  ('00000000-0000-0000-0000-000000000101', now(), now(), 'internal',  'Internal CS engineer'),
  ('00000000-0000-0000-0000-000000000102', now(), now(), 'customer',  'External customer contact')
ON CONFLICT (id) DO NOTHING;

-- Users: one internal CS engineer, one external customer contact.
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user) VALUES
  ('00000000-0000-0000-0000-000000000001', now(), now(), 'seed', 'seed', 'jane.doe@example.com', 'Jane Doe', 'Jane', 'Doe', 'jane.doe@example.com', true, false),
  ('00000000-0000-0000-0000-000000000002', now(), now(), 'seed', 'seed', 'john.smith@example.com', 'John Smith', 'John', 'Smith', 'john.smith@example.com', true, false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO user_role (id, created_on, updated_on, user_id, role_id) VALUES
  ('00000000-0000-0000-0000-000000000201', now(), now(), '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000101'),
  ('00000000-0000-0000-0000-000000000202', now(), now(), '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000102')
ON CONFLICT (id) DO NOTHING;

-- Account + project + deployment.
INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, customer_success_manager_id) VALUES
  ('00000000-0000-0000-0000-000000000301', now(), now(), 'seed', 'seed', 'Example Corp', 'ACC-0001', 'SF-0001', '00000000-0000-0000-0000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, is_active) VALUES
  ('00000000-0000-0000-0000-000000000401', now(), now(), 'seed', 'seed', 'EXCORP-PROJ-1', 'SF-PROJ-0001', 'Example Corp Production', '00000000-0000-0000-0000-000000000301', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id) VALUES
  ('00000000-0000-0000-0000-000000000501', now(), now(), 'seed', 'seed', 'DEP-0001', 'Production', 'PRIMARY_PRODUCTION', true, '00000000-0000-0000-0000-000000000401')
ON CONFLICT (id) DO NOTHING;

-- One case (work_item + case row).
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id, deployment_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000000601', now(), now(), 'seed', 'seed', 'CASE-0001', 'CASE-0001', 'Sample case seeded for local dev', 'CASE', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000501', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed data for docker-compose local dev stack.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO "case" (id, severity, issue_type, state, current_escalation_level, is_escalated, work_state) VALUES
  ('00000000-0000-0000-0000-000000000601', 'S3', 'QUESTION', 'OPEN', 'EL0', false, 'ONGOING')
ON CONFLICT (id) DO NOTHING;

-- One comment on the seeded case.
INSERT INTO comment (id, created_on, created_by, type, work_item_id, content) VALUES
  ('00000000-0000-0000-0000-000000000701', now(), 'jane.doe@example.com', 'COMMENT', '00000000-0000-0000-0000-000000000601', 'This is a seeded comment for local development.')
ON CONFLICT (id) DO NOTHING;

-- One time card entry against the seeded case.
INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, customer_project_id, user_id, work_date, is_billable, state, analyzing_minutes, work_log_comment) VALUES
  ('00000000-0000-0000-0000-000000000801', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000601', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000001', CURRENT_DATE, true, 'SUBMITTED', 30, 'Seeded time card for local dev.')
ON CONFLICT (id) DO NOTHING;

COMMIT;
