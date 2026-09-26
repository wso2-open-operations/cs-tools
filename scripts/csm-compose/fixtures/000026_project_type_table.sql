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

-- LOCAL DEVELOPMENT ONLY.
--
-- project_type is populated from ServiceNow in every real environment, so on a
-- fresh local database the table is empty. Migration
-- 000085_project_type_feature_entitlement then backfills feature entitlements
-- onto named rows and RAISE EXCEPTIONs when it cannot find them -- which stops
-- the migration run dead, taking every later migration with it.
--
-- These are the seven rows that migration names. Supplying them here, straight
-- after the migration that creates the table, lets the real migration run
-- unmodified rather than being skipped or patched.
INSERT INTO project_type (id, created_on, updated_on, created_by, updated_by, name, description, is_active)
VALUES
  ('00000000-0000-0000-0000-0000000000a1', now(), now(), 'local-fixture', 'local-fixture',
   'Managed Cloud Subscription', 'Local dev stand-in for the ServiceNow-sourced row', TRUE),
  ('00000000-0000-0000-0000-0000000000a2', now(), now(), 'local-fixture', 'local-fixture',
   'Evaluation Subscription', 'Local dev stand-in for the ServiceNow-sourced row', TRUE),
  ('00000000-0000-0000-0000-0000000000a3', now(), now(), 'local-fixture', 'local-fixture',
   'Subscription', 'Local dev stand-in for the ServiceNow-sourced row', TRUE),
  ('00000000-0000-0000-0000-0000000000a4', now(), now(), 'local-fixture', 'local-fixture',
   'Cloud Support', 'Local dev stand-in for the ServiceNow-sourced row', TRUE),
  ('00000000-0000-0000-0000-0000000000a5', now(), now(), 'local-fixture', 'local-fixture',
   'Cloud Evaluation Support', 'Local dev stand-in for the ServiceNow-sourced row', TRUE),
  ('00000000-0000-0000-0000-0000000000a6', now(), now(), 'local-fixture', 'local-fixture',
   'Development Support', 'Local dev stand-in for the ServiceNow-sourced row', TRUE),
  ('00000000-0000-0000-0000-0000000000a7', now(), now(), 'local-fixture', 'local-fixture',
   'Professional Services', 'Local dev stand-in for the ServiceNow-sourced row', TRUE)
ON CONFLICT (name) DO NOTHING;
