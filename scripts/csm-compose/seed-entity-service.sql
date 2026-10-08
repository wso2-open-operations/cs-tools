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
-- Idempotent: safe to run more than once. Most rows are ON CONFLICT DO NOTHING;
-- the change request approval personas and the CHG-FIXED-* fixtures at the end
-- are SELF-HEALING instead (upserted / deleted and re-inserted), so an
-- already-seeded database converges to the current fixtures on a re-run -- and
-- the fixtures are put back to their starting state each time (see there).
-- So are the customer portal entitlements of the local project type (the
-- project_type UPDATE below) and the project type of "Lumen Works Platform".

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
INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, customer_success_manager_id, country, city, drive_location) VALUES
  ('00000000-0000-0000-0000-000000000301', now(), now(), 'seed', 'seed', 'Example Corp', 'ACC-0001', 'SF-0001', '00000000-0000-0000-0000-000000000001', 'United States', 'Mountain View', 'https://drive.example.com/example-corp')
ON CONFLICT (id) DO NOTHING;

-- Customer portal entitlements of the local project type. The customer portal shows its
-- Operations menu (Service requests, Change requests) only when the project's type grants
-- read access to them -- GET /projects/{id}/features, read from project_type
-- (has_service_request_read_access / has_change_request_read_access, migration 0130) -- and
-- the local "Subscription" fixture row (a3, the type of the seeded projects below; see
-- fixtures/0031_project_type_table.sql) is created with every flag FALSE, so a customer of
-- project 401 would see no Operations menu and could never reach a change request to
-- approve. (ServiceNow's own "Subscription" does not grant these either; this is a local
-- stand-in, which is why it is only touched while it is still the local fixture row.)
-- An UPDATE, not an INSERT ... ON CONFLICT DO NOTHING: the row already exists on a
-- database seeded earlier, and has to be corrected there too. Only the two flags a
-- customer needs for Operations are set; every other flag of the row, and every other
-- project type, stays as the migration left it.
UPDATE project_type
SET has_change_request_read_access = TRUE,
    has_service_request_read_access = TRUE,
    updated_on = now()
WHERE id = '00000000-0000-0000-0000-0000000000a3'
  AND created_by = 'local-fixture'
  AND (NOT has_change_request_read_access OR NOT has_service_request_read_access);

-- project_type_id is set (a3 = "Subscription", from the project_type fixture) because the
-- customer portal treats a project with no type as "type not loaded" and then offers no
-- deployments in the create-case form.
INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, is_active, project_type_id) VALUES
  ('00000000-0000-0000-0000-000000000401', now(), now(), 'seed', 'seed', 'EXCORP-PROJ-1', 'SF-PROJ-0001', 'Example Corp Production', '00000000-0000-0000-0000-000000000301', true, '00000000-0000-0000-0000-0000000000a3')
ON CONFLICT (id) DO UPDATE SET project_type_id = COALESCE(project.project_type_id, EXCLUDED.project_type_id);

INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id) VALUES
  ('00000000-0000-0000-0000-000000000501', now(), now(), 'seed', 'seed', 'DEP-0001', 'Production', 'PRIMARY_PRODUCTION', true, '00000000-0000-0000-0000-000000000401')
ON CONFLICT (id) DO NOTHING;

-- A second and third deployment for the same project, each with its own
-- deployed products, so the change request form's cascade (Customer Project ->
-- Deployments -> Deployment products) has something to show:
-- Production (PRIMARY_PRODUCTION) runs API Manager 4.3.0 and Identity Server
-- 7.0.0, Staging runs API Manager 4.4.0 only, Development runs nothing yet. A
-- deployment's environment role is its type, so choosing Production + Staging
-- gives three deployment products; choosing Development alone gives none.
INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id) VALUES
  ('00000000-0000-0000-0000-000000000502', now(), now(), 'seed', 'seed', 'DEP-0002', 'Staging',     'STAGING',     true, '00000000-0000-0000-0000-000000000401'),
  ('00000000-0000-0000-0000-000000000503', now(), now(), 'seed', 'seed', 'DEP-0003', 'Development', 'DEVELOPMENT', true, '00000000-0000-0000-0000-000000000401')
ON CONFLICT DO NOTHING;

INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name) VALUES
  ('00000000-0000-0000-0000-000000000511', now(), now(), 'seed', 'seed', 'WSO2', 'SOFTWARE', 'WSO2 API Manager'),
  ('00000000-0000-0000-0000-000000000512', now(), now(), 'seed', 'seed', 'WSO2', 'SOFTWARE', 'WSO2 Identity Server')
ON CONFLICT DO NOTHING;

INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id, current_support_status, release_date) VALUES
  ('00000000-0000-0000-0000-000000000521', now(), now(), 'seed', 'seed', '4.3.0', '00000000-0000-0000-0000-000000000511', 'AVAILABLE', '2024-06-01'),
  ('00000000-0000-0000-0000-000000000522', now(), now(), 'seed', 'seed', '4.4.0', '00000000-0000-0000-0000-000000000511', 'AVAILABLE', '2025-02-01'),
  ('00000000-0000-0000-0000-000000000523', now(), now(), 'seed', 'seed', '7.0.0', '00000000-0000-0000-0000-000000000512', 'AVAILABLE', '2024-03-01')
ON CONFLICT DO NOTHING;

INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, name, active, life_cycle_stage_status, life_cycle_stage, core_count, project_id, deployment_id, product_id, version_id, product_category) VALUES
  ('00000000-0000-0000-0000-000000000531', now(), now(), 'seed', 'seed', 'DP-0001', 'WSO2 API Manager 4.3.0',      true, 'AVAILABLE', 'OPERATIONAL', 8, '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000501', '00000000-0000-0000-0000-000000000511', '00000000-0000-0000-0000-000000000521', 'PDP'),
  ('00000000-0000-0000-0000-000000000532', now(), now(), 'seed', 'seed', 'DP-0002', 'WSO2 Identity Server 7.0.0', true, 'AVAILABLE', 'OPERATIONAL', 4, '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000501', '00000000-0000-0000-0000-000000000512', '00000000-0000-0000-0000-000000000523', 'PDP'),
  ('00000000-0000-0000-0000-000000000533', now(), now(), 'seed', 'seed', 'DP-0003', 'WSO2 API Manager 4.4.0',      true, 'AVAILABLE', 'OPERATIONAL', 4, '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000502', '00000000-0000-0000-0000-000000000511', '00000000-0000-0000-0000-000000000522', 'PDP')
ON CONFLICT DO NOTHING;

-- One case (work_item + case row).
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id, deployment_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000000601', now(), now(), 'seed', 'seed', 'CASE-0001', 'CASE-0001', 'Sample case seeded for local dev', 'CASE', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000501', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed data for docker-compose local dev stack.')
ON CONFLICT (id) DO NOTHING;

-- work_state is NULL: an OPEN case has not been started. ONGOING here would also make
-- jane.doe (its assignee) fail every later "Resume work" with "already has an Ongoing case".
INSERT INTO "case" (id, severity, issue_type, state, current_escalation_level, is_escalated, work_state) VALUES
  ('00000000-0000-0000-0000-000000000601', 'S3', 'QUESTION', 'OPEN', 'EL0', false, NULL)
ON CONFLICT (id) DO NOTHING;

-- One comment on the seeded case.
INSERT INTO comment (id, created_on, created_by, type, work_item_id, content) VALUES
  ('00000000-0000-0000-0000-000000000701', now(), 'jane.doe@example.com', 'COMMENT', '00000000-0000-0000-0000-000000000601', 'This is a seeded comment for local development.')
ON CONFLICT (id) DO NOTHING;

-- One time card entry against the seeded case.
INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, customer_project_id, user_id, work_date, is_billable, state, analyzing_minutes, work_log_comment) VALUES
  ('00000000-0000-0000-0000-000000000801', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000601', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000001', CURRENT_DATE, true, 'SUBMITTED', 30, 'Seeded time card for local dev.')
ON CONFLICT (id) DO NOTHING;

-- One ABT team with the seeded users as members, for GET /teams/{id}/members.
-- key is required (migration 000101_schedule_team_key_catalogue.up.sql added
-- it NOT NULL) -- lower(name), matching that migration's own backfill
-- convention for a row that predates it, since this one is inserted fresh
-- after every migration has already run.
INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type, key) VALUES
  ('00000000-0000-0000-0000-000000000901', now(), now(), 'seed', 'seed', 'Example Corp ABT', 'ABT', 'example corp abt')
ON CONFLICT (id) DO NOTHING;

INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id) VALUES
  ('00000000-0000-0000-0000-000000000902', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000001'),
  ('00000000-0000-0000-0000-000000000903', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002')
ON CONFLICT (id) DO NOTHING;

-- Same id as the team above, in "group" too: account.cre_team_id (what
-- AccountView.CreTeam resolves from) is an FK into "group", not "team" --
-- in a real synced environment both are derived from the same ServiceNow
-- sys_user_group sys_id, so they share an id there too. This is what makes
-- an account's CreTeam.ID usable as GET /teams/{id}/members' teamId.
INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active) VALUES
  ('00000000-0000-0000-0000-000000000901', now(), now(), 'seed', 'seed', 'Example Corp ABT', true)
ON CONFLICT (id) DO NOTHING;

UPDATE account SET cre_team_id = '00000000-0000-0000-0000-000000000901'
WHERE id = '00000000-0000-0000-0000-000000000301';

-- Same team, on the group side: team_member.group_id (distinct from team_id --
-- see entity-service's own CLAUDE.md, "Change requests", "team_member.group_id is
-- the real column for this") is what change_request's approver auto-provisioning
-- reads, not team_id. It is left NULL by the plain INSERT above (and an
-- ON CONFLICT DO NOTHING re-run would never backfill it either), so it is set
-- explicitly here, unconditionally, once the "group" row above exists
-- (group_id's FK requires it) -- same always-apply-regardless-of-conflict shape
-- as the account.cre_team_id UPDATE just above.
--
-- jane.doe stays a member of the TEAM (GET /teams/{id}/members, her own
-- /users/me) but is deliberately NOT in the group (group_id NULL): she is the
-- requester persona of the change request fixtures, not one of their approvers.
-- john.smith (a customer) stays in the group on purpose: he is the standing
-- probe of the INTERNAL-only approver pools -- an external member of the
-- assigned group who must never be provisioned as an approver of an internal
-- stage (see "Approver pools" in entity-service's CLAUDE.md).
UPDATE team_member SET group_id = NULL
WHERE id = '00000000-0000-0000-0000-000000000902';
UPDATE team_member SET group_id = '00000000-0000-0000-0000-000000000901'
WHERE id = '00000000-0000-0000-0000-000000000903';

-- ---------------------------------------------------------------------------
-- Change request approval personas: a separate set of people to exercise the
-- approval and customer-approval flows, so the fixtures below do not hang on
-- jane.doe / john.smith (whose rows stay -- cases, time cards and the other
-- seed data use them).
--
--   WSO2 staff (INTERNAL, role 'internal') -- the approvers of the internal
--   stages (Peer, CAB, Review; an Emergency change has the one CAB stage):
--     alice.perera@example.com    Alice Perera
--     bob.fernando@example.com    Bob Fernando
--     carol.silva@example.com     Carol Silva
--   They are members of "Example Corp ABT" (group 901, the assigned group of
--   every fixture below), of "CAB Approval" and of "Devops Approval" (the peer
--   approval fallback group, seeded here so the fallback can be exercised
--   locally). The membership of those two groups is a data matter (an operations
--   matter: there is no screen for it): without rows like these, Request Approval on a
--   Normal change (its CAB stage) or an Emergency change (its only stage, also the
--   CAB's) is refused with "the \"CAB Approval\" group has no members ...".
--
--   Customers (EXTERNAL, role 'customer') -- registered contacts of project 401
--   "Example Corp Production", i.e. the Customer Group asked at Customer
--   Approval / Customer Review:
--     dave.mendis@example.com        Dave Mendis
--     erin.jayawardena@example.com   Erin Jayawardena
--
-- "user".user_type is derived from the roles by recompute_user_type() (trigger
-- on user_role), so these are INTERNAL / EXTERNAL without being set by hand.
-- Two traps, both from seed-team-schedule.sql's own notes: never grant the
-- internal role to a customer (recompute checks internal before external, so
-- that customer would become INTERNAL and be handed unrestricted scope over
-- every project), and never give these grants created_by = 'seed' -- that file
-- deletes every 'seed' internal grant of anyone outside its own roster.
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user) VALUES
  ('00000000-0000-0000-0000-000000000011', now(), now(), 'seed', 'seed', 'alice.perera@example.com',       'Alice Perera',       'Alice', 'Perera',     'alice.perera@example.com',       true, false),
  ('00000000-0000-0000-0000-000000000012', now(), now(), 'seed', 'seed', 'bob.fernando@example.com',       'Bob Fernando',       'Bob',   'Fernando',   'bob.fernando@example.com',       true, false),
  ('00000000-0000-0000-0000-000000000013', now(), now(), 'seed', 'seed', 'carol.silva@example.com',        'Carol Silva',        'Carol', 'Silva',      'carol.silva@example.com',        true, false),
  ('00000000-0000-0000-0000-000000000021', now(), now(), 'seed', 'seed', 'dave.mendis@example.com',        'Dave Mendis',        'Dave',  'Mendis',     'dave.mendis@example.com',        true, false),
  ('00000000-0000-0000-0000-000000000022', now(), now(), 'seed', 'seed', 'erin.jayawardena@example.com',   'Erin Jayawardena',   'Erin',  'Jayawardena','erin.jayawardena@example.com',   true, false)
ON CONFLICT (id) DO NOTHING;

-- No created_by on purpose (see above).
INSERT INTO user_role (id, created_on, updated_on, user_id, role_id) VALUES
  ('00000000-0000-0000-0000-000000000211', now(), now(), '00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000101'),
  ('00000000-0000-0000-0000-000000000212', now(), now(), '00000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000101'),
  ('00000000-0000-0000-0000-000000000213', now(), now(), '00000000-0000-0000-0000-000000000013', '00000000-0000-0000-0000-000000000101'),
  ('00000000-0000-0000-0000-000000000221', now(), now(), '00000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000102'),
  ('00000000-0000-0000-0000-000000000222', now(), now(), '00000000-0000-0000-0000-000000000022', '00000000-0000-0000-0000-000000000102')
ON CONFLICT (id) DO NOTHING;
-- Self-healing: a customer persona never holds the internal role.
DELETE FROM user_role
WHERE user_id IN ('00000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000022')
  AND role_id = '00000000-0000-0000-0000-000000000101';

-- Example Corp ABT (group 901): the three internal personas. team_member.team_id
-- is NOT NULL, so the team fills it; the membership that counts is group_id.
INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id) VALUES
  ('00000000-0000-0000-0000-000000000904', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000901'),
  ('00000000-0000-0000-0000-000000000905', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000901'),
  ('00000000-0000-0000-0000-000000000906', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000013', '00000000-0000-0000-0000-000000000901')
ON CONFLICT (id) DO UPDATE SET team_id = EXCLUDED.team_id, user_id = EXCLUDED.user_id, group_id = EXCLUDED.group_id;

-- CAB Approval membership (migration 0188 creates the group, empty). A Normal
-- change cannot be sent for approval, and its peer approval cannot cascade to CAB,
-- unless the CAB group has an eligible (active, internal) member -- and an Emergency
-- change, whose ONE stage is the CAB's too (the previous system has no Emergency CAB), cannot
-- be sent for approval at all -- so the three internal personas sit in the group,
-- which lets the fixtures run the full Request Approval -> Peer -> CAB -> Scheduled
-- (Normal) and Request Approval -> CAB -> Scheduled (Emergency) flows locally.
-- (jane.doe and john.smith held these seats before; their rows 1101-1104 are
-- removed so an already-seeded database converges. Rows 1121-1123 seated the same
-- three personas in the "ECAB Approval" group migration 0188 also created; nothing
-- resolves that group any more, so they are removed too, and the group is left
-- as it is.)
DELETE FROM team_member WHERE id IN (
  '00000000-0000-0000-0000-000000001101', '00000000-0000-0000-0000-000000001102',
  '00000000-0000-0000-0000-000000001103', '00000000-0000-0000-0000-000000001104',
  '00000000-0000-0000-0000-000000001121', '00000000-0000-0000-0000-000000001122',
  '00000000-0000-0000-0000-000000001123');
INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id) VALUES
  ('00000000-0000-0000-0000-000000001111', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000011', '00000000-0000-4000-8000-00000000ca01'),
  ('00000000-0000-0000-0000-000000001112', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000012', '00000000-0000-4000-8000-00000000ca01'),
  ('00000000-0000-0000-0000-000000001113', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000013', '00000000-0000-4000-8000-00000000ca01')
ON CONFLICT (id) DO UPDATE SET team_id = EXCLUDED.team_id, user_id = EXCLUDED.user_id, group_id = EXCLUDED.group_id;

-- Devops Approval: the peer approval fallback group (domain.PeerApprovalFallbackGroupName),
-- used when a Normal change has no assigned group or its assigned group yields
-- no eligible member. Not created by any migration, so it is seeded here -- only
-- when no group of that name exists (a synced environment may already mirror
-- one, and a second must not be added), and its members attach to whichever
-- group of that name is found first.
INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, description, is_active)
SELECT '00000000-0000-4000-8000-0000000de001'::uuid, now(), now(), 'seed', 'seed',
       'Devops Approval', 'Peer approval fallback group: the peer approvers of a Normal change whose assigned group yields nobody.', true
WHERE NOT EXISTS (SELECT 1 FROM "group" WHERE name = 'Devops Approval')
ON CONFLICT (id) DO NOTHING;

INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
SELECT m.id, now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901'::uuid, m.user_id,
       (SELECT g.id FROM "group" g WHERE g.name = 'Devops Approval' ORDER BY g.created_on, g.id LIMIT 1)
FROM (VALUES
  ('00000000-0000-0000-0000-000000001131'::uuid, '00000000-0000-0000-0000-000000000011'::uuid),
  ('00000000-0000-0000-0000-000000001132'::uuid, '00000000-0000-0000-0000-000000000012'::uuid),
  ('00000000-0000-0000-0000-000000001133'::uuid, '00000000-0000-0000-0000-000000000013'::uuid)
) AS m(id, user_id)
ON CONFLICT (id) DO UPDATE SET user_id = EXCLUDED.user_id, group_id = EXCLUDED.group_id;

-- An earlier version of this seed modelled the Customer Group as a "group" of
-- members ("Example Corp Customer Approvers", 911; 912 for the second customer)
-- with jane.doe / john.smith in it. The Customer Group is now derived from the
-- project's registered contacts (below) and these groups are referenced by
-- nothing, so remove them -- and their memberships -- from a database that was
-- seeded by that version. (No-op on a fresh one.)
DELETE FROM team_member WHERE group_id IN (
  '00000000-0000-0000-0000-000000000911', '00000000-0000-0000-0000-000000000912');
DELETE FROM "group" WHERE id IN (
  '00000000-0000-0000-0000-000000000911', '00000000-0000-0000-0000-000000000912');

-- Customer Group: the registered contacts of the change request's Customer
-- Project, derived live and read-only (entity-service CLAUDE.md, "Customer
-- Group"). A contact is a project_contact in state REGISTERED holding the
-- PORTAL_USER project role (through project_contact_group -> project_group ->
-- project_group_role -> project_role) whose "user" is active; they are the
-- people asked at Customer Approval / Customer Review. Two customers are seeded
-- so the isolation is demonstrable:
--   Example Corp (account 301)  project 401 "Example Corp Production"
--                               <- registered contacts dave.mendis, erin.jayawardena
--   Other Corp   (account 302)  project 402 "Other Corp Production"
--                               <- registered contact sam.other
-- A change request of project 401 is only ever put to dave / erin, one of
-- project 402 only to sam.other. (project_role / project_group rows are
-- normally synced from ServiceNow; they are seeded here for the local stack.)
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user) VALUES
  ('00000000-0000-0000-0000-000000000003', now(), now(), 'seed', 'seed', 'sam.other@othercorp.example', 'Sam Other', 'Sam', 'Other', 'sam.other@othercorp.example', true, false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, customer_success_manager_id, country, city, drive_location) VALUES
  ('00000000-0000-0000-0000-000000000302', now(), now(), 'seed', 'seed', 'Other Corp', 'ACC-0002', 'SF-0002', '00000000-0000-0000-0000-000000000001', 'United Kingdom', 'London', 'https://drive.example.com/other-corp')
ON CONFLICT (id) DO NOTHING;

INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, is_active, project_type_id) VALUES
  ('00000000-0000-0000-0000-000000000402', now(), now(), 'seed', 'seed', 'OTHERCORP-PROJ-1', 'SF-PROJ-0002', 'Other Corp Production', '00000000-0000-0000-0000-000000000302', true, '00000000-0000-0000-0000-0000000000a3')
ON CONFLICT (id) DO UPDATE SET project_type_id = COALESCE(project.project_type_id, EXCLUDED.project_type_id);

INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id) VALUES
  ('00000000-0000-0000-0000-000000000541', now(), now(), 'seed', 'seed', 'DEP-0010', 'Other Corp Production', 'PRIMARY_PRODUCTION', true, '00000000-0000-0000-0000-000000000402')
ON CONFLICT (id) DO NOTHING;

-- The PORTAL_USER role and the "General Access" project group that carries it.
INSERT INTO project_role (id, created_on, updated_on, created_by, updated_by, role) VALUES
  ('00000000-0000-0000-0000-000000001401', now(), now(), 'seed', 'seed', 'PORTAL_USER')
ON CONFLICT (role) DO NOTHING;
INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group") VALUES
  ('00000000-0000-0000-0000-000000001402', now(), now(), 'seed', 'seed', 'General Access')
ON CONFLICT ("group") DO NOTHING;
INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
SELECT '00000000-0000-0000-0000-000000001403', now(), now(), 'seed', 'seed', pg.id, pr.id
FROM project_group pg, project_role pr
WHERE pg."group" = 'General Access' AND pr.role = 'PORTAL_USER'
ON CONFLICT DO NOTHING;

-- jane.doe and john.smith were registered contacts of project 401 (rows
-- 1421/1422 and their project_contact_group links 1431/1432) before the
-- personas above; they no longer are. Their account_contact rows (1411/1412)
-- stay: they are still contacts of the Example Corp ACCOUNT, just not
-- registered on a project. Self-healing for an already-seeded database.
DELETE FROM project_contact_group WHERE id IN (
  '00000000-0000-0000-0000-000000001431', '00000000-0000-0000-0000-000000001432');
DELETE FROM project_contact WHERE id IN (
  '00000000-0000-0000-0000-000000001421', '00000000-0000-0000-0000-000000001422');

INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, is_active, user_name, account_id) VALUES
  ('00000000-0000-0000-0000-000000001411', now(), now(), 'seed', 'seed', true, 'jane.doe@example.com', '00000000-0000-0000-0000-000000000301'),
  ('00000000-0000-0000-0000-000000001412', now(), now(), 'seed', 'seed', true, 'john.smith@example.com', '00000000-0000-0000-0000-000000000301'),
  ('00000000-0000-0000-0000-000000001413', now(), now(), 'seed', 'seed', true, 'sam.other@othercorp.example', '00000000-0000-0000-0000-000000000302'),
  ('00000000-0000-0000-0000-000000001414', now(), now(), 'seed', 'seed', true, 'dave.mendis@example.com', '00000000-0000-0000-0000-000000000301'),
  ('00000000-0000-0000-0000-000000001415', now(), now(), 'seed', 'seed', true, 'erin.jayawardena@example.com', '00000000-0000-0000-0000-000000000301')
ON CONFLICT (id) DO NOTHING;

INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, state, account_contact_id, project_id) VALUES
  ('00000000-0000-0000-0000-000000001423', now(), now(), 'seed', 'seed', 'sam.other@othercorp.example', 'REGISTERED', '00000000-0000-0000-0000-000000001413', '00000000-0000-0000-0000-000000000402'),
  ('00000000-0000-0000-0000-000000001424', now(), now(), 'seed', 'seed', 'dave.mendis@example.com', 'REGISTERED', '00000000-0000-0000-0000-000000001414', '00000000-0000-0000-0000-000000000401'),
  ('00000000-0000-0000-0000-000000001425', now(), now(), 'seed', 'seed', 'erin.jayawardena@example.com', 'REGISTERED', '00000000-0000-0000-0000-000000001415', '00000000-0000-0000-0000-000000000401')
ON CONFLICT (id) DO UPDATE SET state = 'REGISTERED', project_id = EXCLUDED.project_id, account_contact_id = EXCLUDED.account_contact_id;

INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
SELECT c.id, now(), now(), 'seed', 'seed', c.contact, pg.id
FROM (VALUES
  ('00000000-0000-0000-0000-000000001433'::uuid, '00000000-0000-0000-0000-000000001423'::uuid),
  ('00000000-0000-0000-0000-000000001434'::uuid, '00000000-0000-0000-0000-000000001424'::uuid),
  ('00000000-0000-0000-0000-000000001435'::uuid, '00000000-0000-0000-0000-000000001425'::uuid)
) AS c(id, contact), project_group pg
WHERE pg."group" = 'General Access'
ON CONFLICT (id) DO NOTHING;

-- "Lumen Works Platform": registered customer contacts for the generated project.
-- The seed-generator (a separate, randomised pass that runs AFTER this script)
-- creates projects with random names; "Lumen Works Platform" is one of them, with
-- generated contacts that do not qualify as a Customer Group (no "user" row, no
-- PORTAL_USER project role). So that a change request on that project has
-- customer approvers, two real customer users are registered on it here:
--     mira.santos@lumenworks.example   Mira Santos
--     noel.prasad@lumenworks.example   Noel Prasad
-- They meet the same criteria as the Example Corp contacts above: a REGISTERED
-- project_contact holding the PORTAL_USER role (through the "General Access"
-- project group), whose "user" is active and a customer (role 'customer', so
-- EXTERNAL; never the internal role). The project is found BY NAME because its id
-- is random per database; where no project of that name exists this block does
-- nothing, and it takes effect on the next seed run after the generator has
-- created it (`docker-compose up -d migrate` re-runs this script).
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user) VALUES
  ('00000000-0000-0000-0000-000000000023', now(), now(), 'seed', 'seed', 'mira.santos@lumenworks.example', 'Mira Santos', 'Mira', 'Santos', 'mira.santos@lumenworks.example', true, false),
  ('00000000-0000-0000-0000-000000000024', now(), now(), 'seed', 'seed', 'noel.prasad@lumenworks.example', 'Noel Prasad', 'Noel', 'Prasad', 'noel.prasad@lumenworks.example', true, false)
ON CONFLICT (id) DO NOTHING;

-- The generator also gives every project a RANDOM project type, and only some types let a
-- customer see Operations in the customer portal (see the project_type entitlements near the
-- top): "Lumen Works Platform" can come out as e.g. "Cloud Support", whose customers (mira,
-- noel) would then see no Change requests at all. So a Lumen Works Platform whose type does
-- not grant change request read access is put on "Subscription" (a3). A type that already
-- grants it ("Managed Cloud Subscription", or a3 itself) is left alone.
UPDATE project
SET project_type_id = '00000000-0000-0000-0000-0000000000a3', updated_on = now()
WHERE id = (
    SELECT id FROM project
    WHERE name = 'Lumen Works Platform' AND account_id IS NOT NULL
    ORDER BY created_on, id LIMIT 1)
  AND NOT EXISTS (
    SELECT 1 FROM project_type pt
    WHERE pt.id = project.project_type_id AND pt.has_change_request_read_access);

-- No created_by on purpose (see the personas above).
INSERT INTO user_role (id, created_on, updated_on, user_id, role_id) VALUES
  ('00000000-0000-0000-0000-000000000223', now(), now(), '00000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000102'),
  ('00000000-0000-0000-0000-000000000224', now(), now(), '00000000-0000-0000-0000-000000000024', '00000000-0000-0000-0000-000000000102')
ON CONFLICT (id) DO NOTHING;
DELETE FROM user_role
WHERE user_id IN ('00000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000024')
  AND role_id = '00000000-0000-0000-0000-000000000101';

WITH lumen AS (
  SELECT id, account_id FROM project
  WHERE name = 'Lumen Works Platform' AND account_id IS NOT NULL
  ORDER BY created_on, id LIMIT 1
)
INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, is_active, user_name, account_id)
SELECT c.id, now(), now(), 'seed', 'seed', true, c.user_name, lumen.account_id
FROM lumen, (VALUES
  ('00000000-0000-0000-0000-000000001416'::uuid, 'mira.santos@lumenworks.example'),
  ('00000000-0000-0000-0000-000000001417'::uuid, 'noel.prasad@lumenworks.example')
) AS c(id, user_name)
ON CONFLICT (id) DO UPDATE SET account_id = EXCLUDED.account_id, is_active = true;

WITH lumen AS (
  SELECT id FROM project
  WHERE name = 'Lumen Works Platform' AND account_id IS NOT NULL
  ORDER BY created_on, id LIMIT 1
)
INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, state, account_contact_id, project_id)
SELECT c.id, now(), now(), 'seed', 'seed', c.email, 'REGISTERED', c.account_contact_id, lumen.id
FROM lumen, (VALUES
  ('00000000-0000-0000-0000-000000001426'::uuid, 'mira.santos@lumenworks.example', '00000000-0000-0000-0000-000000001416'::uuid),
  ('00000000-0000-0000-0000-000000001427'::uuid, 'noel.prasad@lumenworks.example', '00000000-0000-0000-0000-000000001417'::uuid)
) AS c(id, email, account_contact_id)
ON CONFLICT (id) DO UPDATE SET state = 'REGISTERED', project_id = EXCLUDED.project_id, account_contact_id = EXCLUDED.account_contact_id;

INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
SELECT c.id, now(), now(), 'seed', 'seed', c.contact, pg.id
FROM (VALUES
  ('00000000-0000-0000-0000-000000001436'::uuid, '00000000-0000-0000-0000-000000001426'::uuid),
  ('00000000-0000-0000-0000-000000001437'::uuid, '00000000-0000-0000-0000-000000001427'::uuid)
) AS c(id, contact), project_group pg
WHERE pg."group" = 'General Access'
  AND EXISTS (SELECT 1 FROM project_contact pc WHERE pc.id = c.contact)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Change request fixtures: eight fixed-UUID change requests (block 1001+) giving
-- Playwright E2E specs a real, deterministic change request to navigate straight
-- to by id/number at every stage of its lifecycle, since the webapp's own
-- "Create Change Request" flow always 503s on this data source (work_item.number
-- has no DB sequence -- see entity-service's own CLAUDE.md, "CreateCase and case
-- numbers"/"Change requests"). Each is a work_item (type CHANGE_REQUEST) +
-- change_request row, the same shared-primary-key extension shape as the
-- "case"/work_item pair above. wso2_id is left unset: it's nullable for change
-- requests specifically (migration 0021's own comment -- "change_request work
-- items have no wso2_id data"), unlike the case fixture above, whose type
-- requires it.
--
-- SELF-HEALING: these fixtures are driven forward by the specs (a request for
-- approval, an approval, a customer decision), and the plain
-- ON CONFLICT DO NOTHING every other row in this file uses would leave a
-- mutated row mutated -- and an already-seeded database holding the pre-persona
-- fixtures (jane.doe / john.smith as approvers and contacts) would never pick up
-- the new ones. So running this file RESETS them: every approval stage and
-- approver of the eight is deleted and re-inserted below, and the work_item /
-- change_request rows are upserted back to their starting state (state, owner
-- fields, customer flags and stamps, schedule and hold columns). Re-running the
-- seed (`docker-compose up -d migrate`) is therefore the way to put the
-- fixtures back; the Playwright suite does exactly that before it starts.
--
--   CHG-FIXED-001  NEW, no assignment group: "Request Approval is blocked until a
--                  team is assigned". requested_by_user_id = john.smith.
--   CHG-FIXED-002  NEW, assignment group 901: Request Approval provisions the
--                  Peer Approval stage for 901's active INTERNAL members (alice,
--                  bob, carol); john.smith, an external member of the group, is
--                  skipped. requested_by_user_id is NULL on this and the fixtures
--                  below so nobody is excluded as the creator.
--   CHG-FIXED-003  ASSESS, Peer Approval stage with alice, bob, carol REQUESTED:
--                  the first to approve cascades to Authorize (CAB Approval) and
--                  cancels the siblings.
--   CHG-FIXED-004  AUTHORIZE, Peer Approval already decided (alice approved, bob
--                  and carol cancelled).
--   CHG-FIXED-005  Standard, NEW, Customer Approval ticked: Request Approval
--                  lands in Customer Approval, where project 401's contacts (dave,
--                  erin) are asked.
--   CHG-FIXED-006  Normal, REVIEW, Customer Review ticked: Review offers
--                  customer_review instead of closed.
--   CHG-FIXED-007  Normal, CUSTOMER_APPROVAL on project 401 with its Customer
--                  Approval stage provisioned: dave and erin REQUESTED; the first
--                  to approve schedules the change, a rejection cancels it.
--   CHG-FIXED-008  CUSTOMER_REVIEW on project 401 with its Customer Review stage
--                  provisioned: dave and erin REQUESTED; approving closes it,
--                  rejecting moves it to Rollback.
-- The customer stages have no assignment group (the Customer Group is the
-- project's contacts, not a "group" row) and carry an explicit checkpoint_label
-- (migration 0179), which is how they are recognised.
--
-- WHAT A CUSTOMER SEES OF THESE (entity-service/CLAUDE.md, "Customer visibility and
-- the cutover"; the local stack runs with CR_STRICT_VISIBILITY_FROM far in the
-- past, so every fixture here is strict): a change request is visible to a customer
-- only when it was DESIGNATED to them -- they hold an approver row, in any state, on
-- one of its "Customer Approval" / "Customer Review" stages. That is exactly the
-- approver rows of CHG-FIXED-007 and -008 below: dave and erin see those two, in every
-- state the specs walk them through (their rows turn APPROVED / REJECTED / CANCELLED,
-- never away). Every other fixture -- New, Assess, Authorize, the Review one with
-- Customer Review ticked but not yet asked -- has no customer-stage row, so a customer
-- does not see it (absent from the list and its counts, 404 by id). A fixture that must
-- be visible to a customer needs a customer-stage approver row for them (CANCELLED
-- counts: it is what the sibling of a contact who answered holds).
DELETE FROM approval_stage_approver WHERE work_item_id IN (
  '00000000-0000-0000-0000-000000001001', '00000000-0000-0000-0000-000000001002',
  '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000001004',
  '00000000-0000-0000-0000-000000001201', '00000000-0000-0000-0000-000000001202',
  '00000000-0000-0000-0000-000000001303', '00000000-0000-0000-0000-000000001304');
DELETE FROM approval_stage WHERE work_item_id IN (
  '00000000-0000-0000-0000-000000001001', '00000000-0000-0000-0000-000000001002',
  '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000001004',
  '00000000-0000-0000-0000-000000001201', '00000000-0000-0000-0000-000000001202',
  '00000000-0000-0000-0000-000000001303', '00000000-0000-0000-0000-000000001304');

INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id, assignment_group_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000001001', now(), now(), 'seed', 'seed', 'CHG-FIXED-001', 'E2E fixture: New change request with no assigned team', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', NULL, '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: New state, no assignment_group_id -- Move to Assess must be blocked until a team is assigned.'),
  ('00000000-0000-0000-0000-000000001002', now(), now(), 'seed', 'seed', 'CHG-FIXED-002', 'E2E fixture: New change request with an assigned team', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: New state, assignment_group_id set to the seeded Example Corp ABT group -- Move to Assess must succeed and auto-provision that team''s active internal members (alice, bob, carol) as peer approvers; john.smith, a customer in the group, is skipped.'),
  ('00000000-0000-0000-0000-000000001003', now(), now(), 'seed', 'seed', 'CHG-FIXED-003', 'E2E fixture: change request in Assess with three pending approvers', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: Assess state with three requested peer approvers (alice, bob, carol) on one stage -- approving one must cascade to Authorize and cancel the others.'),
  ('00000000-0000-0000-0000-000000001004', now(), now(), 'seed', 'seed', 'CHG-FIXED-004', 'E2E fixture: change request in Authorize with a resolved approval', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: Authorize state with a resolved peer approval (alice approved, bob and carol cancelled) -- a terminal approval decision already applied.'),
  ('00000000-0000-0000-0000-000000001201', now(), now(), 'seed', 'seed', 'CHG-FIXED-005', 'E2E fixture: Standard change requiring customer approval', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: Standard change in New with the Customer Approval checkbox ticked -- Request Approval must land in Customer Approval, where the project''s contacts (dave, erin) are asked.'),
  ('00000000-0000-0000-0000-000000001202', now(), now(), 'seed', 'seed', 'CHG-FIXED-006', 'E2E fixture: change in Review requiring customer review', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: Review state with the Customer Review checkbox ticked -- Review must offer customer_review instead of closed.'),
  ('00000000-0000-0000-0000-000000001303', now(), now(), 'seed', 'seed', 'CHG-FIXED-007', 'E2E fixture: change in Customer Approval with a pending customer group approval', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture: Customer Approval state on project 401, Customer Approval stage with two requested approvers (the project''s registered contacts dave.mendis, erin.jayawardena).'),
  ('00000000-0000-0000-0000-000000001304', now(), now(), 'seed', 'seed', 'CHG-FIXED-008', 'E2E fixture: change in Customer Review with a pending customer group review', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture: Customer Review state on project 401, Customer Review stage with two requested approvers (the project''s registered contacts dave.mendis, erin.jayawardena).')
ON CONFLICT (id) DO UPDATE SET
  number = EXCLUDED.number, subject = EXCLUDED.subject, description = EXCLUDED.description,
  account_id = EXCLUDED.account_id, project_id = EXCLUDED.project_id,
  assignment_group_id = EXCLUDED.assignment_group_id,
  opened_by_user_id = EXCLUDED.opened_by_user_id, assigned_to_id = EXCLUDED.assigned_to_id,
  updated_on = now();

INSERT INTO change_request (id, state, change_model, priority, impact, category, risk, change_request_type, requested_by_user_id, justification, customer_approval_required, customer_review_required) VALUES
  ('00000000-0000-0000-0000-000000001001', 'NEW'::change_request_state_enum, NULL, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, '00000000-0000-0000-0000-000000000002', 'Seed fixture: no team assigned, used to cover the compulsory-team gate before Assess.', false, false),
  ('00000000-0000-0000-0000-000000001002', 'NEW'::change_request_state_enum, NULL, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: team assigned, used to cover Assess-entry approver auto-provisioning.', false, false),
  ('00000000-0000-0000-0000-000000001003', 'ASSESS'::change_request_state_enum, NULL, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Assess state, approval already provisioned, used to cover the approve/cancel-sibling cascade.', false, false),
  ('00000000-0000-0000-0000-000000001004', 'AUTHORIZE'::change_request_state_enum, NULL, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Authorize state, approval already decided, used to cover a terminal-approval-state display.', false, false),
  ('00000000-0000-0000-0000-000000001201', 'NEW'::change_request_state_enum, 'STANDARD'::change_request_change_model_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Standard change, Customer Approval ticked.', true, false),
  ('00000000-0000-0000-0000-000000001202', 'REVIEW'::change_request_state_enum, 'NORMAL'::change_request_change_model_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Review state, Customer Review ticked.', false, true),
  ('00000000-0000-0000-0000-000000001303', 'CUSTOMER_APPROVAL'::change_request_state_enum, 'NORMAL'::change_request_change_model_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Customer Approval with the project''s customer contacts.', true, false),
  ('00000000-0000-0000-0000-000000001304', 'CUSTOMER_REVIEW'::change_request_state_enum, 'NORMAL'::change_request_change_model_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Customer Review with the project''s customer contacts.', false, true)
ON CONFLICT (id) DO UPDATE SET
  state = EXCLUDED.state, change_model = EXCLUDED.change_model, priority = EXCLUDED.priority,
  impact = EXCLUDED.impact, category = EXCLUDED.category, risk = EXCLUDED.risk,
  change_request_type = EXCLUDED.change_request_type,
  requested_by_user_id = EXCLUDED.requested_by_user_id, justification = EXCLUDED.justification,
  customer_approval_required = EXCLUDED.customer_approval_required,
  customer_review_required = EXCLUDED.customer_review_required,
  -- whatever a spec or a manual walk-through stamped on the way (a customer's proposed time and WSO2's answer to it included)
  approval = NULL, is_customer_approval_required = NULL, is_customer_review_required = NULL,
  start_on = NULL, end_on = NULL, closed_by_user_id = NULL, closed_on = NULL,
  is_on_hold = NULL, on_hold_reason = NULL, customer_group_id = NULL,
  customer_updated_on = NULL, customer_updated_date_confirmation = NULL;

INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label) VALUES
  ('00000000-0000-0000-0000-000000001005', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000901', 'REQUESTED', 'Peer Approval'),
  ('00000000-0000-0000-0000-000000001008', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000901', 'APPROVED', 'Peer Approval'),
  ('00000000-0000-0000-0000-000000001305', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001303', NULL, 'REQUESTED', 'Customer Approval'),
  ('00000000-0000-0000-0000-000000001306', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001304', NULL, 'REQUESTED', 'Customer Review');

-- approval_stage_approver.state stores UPPER_SNAKE_CASE values after migration
-- 0138 renamed the column from status and normalised existing rows.
INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state) VALUES
  -- CHG-FIXED-003: alice, bob, carol all REQUESTED
  ('00000000-0000-0000-0000-000000001006', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000011', 'REQUESTED'),
  ('00000000-0000-0000-0000-000000001007', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000012', 'REQUESTED'),
  ('00000000-0000-0000-0000-000000001011', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000013', 'REQUESTED'),
  -- CHG-FIXED-004: alice approved, bob and carol cancelled
  ('00000000-0000-0000-0000-000000001009', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001008', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000011', 'APPROVED'),
  ('00000000-0000-0000-0000-000000001010', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001008', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000012', 'CANCELLED'),
  ('00000000-0000-0000-0000-000000001012', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001008', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000013', 'CANCELLED'),
  -- CHG-FIXED-007 (Customer Approval) / CHG-FIXED-008 (Customer Review): dave, erin REQUESTED
  ('00000000-0000-0000-0000-000000001307', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001305', '00000000-0000-0000-0000-000000001303', '00000000-0000-0000-0000-000000000021', 'REQUESTED'),
  ('00000000-0000-0000-0000-000000001308', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001305', '00000000-0000-0000-0000-000000001303', '00000000-0000-0000-0000-000000000022', 'REQUESTED'),
  ('00000000-0000-0000-0000-000000001309', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001306', '00000000-0000-0000-0000-000000001304', '00000000-0000-0000-0000-000000000021', 'REQUESTED'),
  ('00000000-0000-0000-0000-000000001310', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001306', '00000000-0000-0000-0000-000000001304', '00000000-0000-0000-0000-000000000022', 'REQUESTED');

COMMIT;
