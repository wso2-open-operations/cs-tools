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
INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, customer_success_manager_id, country, city, drive_location) VALUES
  ('00000000-0000-0000-0000-000000000301', now(), now(), 'seed', 'seed', 'Example Corp', 'ACC-0001', 'SF-0001', '00000000-0000-0000-0000-000000000001', 'United States', 'Mountain View', 'https://drive.example.com/example-corp')
ON CONFLICT (id) DO NOTHING;

-- project_type_id is set (a3 = "Subscription", from the project_type fixture) because the
-- customer portal treats a project with no type as "type not loaded" and then offers no
-- deployments in the create-case form.
INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, is_active, project_type_id) VALUES
  ('00000000-0000-0000-0000-000000000401', now(), now(), 'seed', 'seed', 'EXCORP-PROJ-1', 'SF-PROJ-0001', 'Example Corp Production', '00000000-0000-0000-0000-000000000301', true, '00000000-0000-0000-0000-0000000000a3')
ON CONFLICT (id) DO UPDATE SET project_type_id = COALESCE(project.project_type_id, EXCLUDED.project_type_id);

INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id) VALUES
  ('00000000-0000-0000-0000-000000000501', now(), now(), 'seed', 'seed', 'DEP-0001', 'Production', 'PRIMARY_PRODUCTION', true, '00000000-0000-0000-0000-000000000401')
ON CONFLICT (id) DO NOTHING;

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

-- One ABT team with both seeded users as members, for GET /teams/{id}/members.
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

-- team_member.group_id (distinct from team_id -- see entity-service's own CLAUDE.md,
-- "Change requests" section, "team_member.group_id is the real column for
-- this") is what change_request's Assess-entry approver auto-provisioning
-- actually reads, not team_id. It's left NULL by the plain INSERT above (and
-- an ON CONFLICT DO NOTHING re-run would never backfill it either), so it's
-- set explicitly here, unconditionally, once the "group" row above exists
-- (group_id's FK requires it) -- same always-apply-regardless-of-conflict
-- shape as the account.cre_team_id UPDATE just above. This makes both seeded
-- users resolve as members of "group" 901, which CR-FIXED-002's own
-- {state: "assess"} exercise (see the change-request fixtures below) relies
-- on to auto-provision them as approvers.
UPDATE team_member SET group_id = '00000000-0000-0000-0000-000000000901'
WHERE id IN ('00000000-0000-0000-0000-000000000902', '00000000-0000-0000-0000-000000000903');

-- Change requests: four fixed-UUID fixtures (block 1001+) giving Playwright
-- E2E specs a real, deterministic change request to navigate straight to by
-- id/number at every stage of its lifecycle, since the webapp's own "Create
-- Change Request" flow always 503s on this data source (work_item.number has
-- no DB sequence -- see entity-service's own CLAUDE.md, "CreateCase and case
-- numbers"/"Change requests"). Each is a work_item (type CHANGE_REQUEST) +
-- change_request row, the same shared-primary-key extension shape as the
-- "case"/work_item pair above. wso2_id is left unset: it's nullable for
-- change requests specifically (migration 0021's own comment -- "change_request
-- work items have no wso2_id data"), unlike the case fixture above, whose
-- type requires it.
--
-- CR-FIXED-001: NEW, no assignment_group_id at all -- covers "Move to Assess
-- is disabled/blocked with no team assigned" (PatchChangeRequest's compulsory-
-- team gate, entity-service's own CLAUDE.md).
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000001001', now(), now(), 'seed', 'seed', 'CHG-FIXED-001', 'E2E fixture: New change request with no assigned team', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: New state, no assignment_group_id -- Move to Assess must be blocked until a team is assigned.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO change_request (id, state, priority, impact, category, risk, change_request_type, requested_by_user_id, justification) VALUES
  ('00000000-0000-0000-0000-000000001001', 'NEW'::change_request_state_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, '00000000-0000-0000-0000-000000000002', 'Seed fixture: no team assigned, used to cover the compulsory-team gate before Assess.')
ON CONFLICT (id) DO NOTHING;

-- CR-FIXED-002: NEW, assignment_group_id = the existing "Example Corp ABT"
-- group (901) -- covers "Move to Assess succeeds once a team is assigned, and
-- auto-provisions that team's members (jane.doe/john.smith, via the
-- team_member.group_id UPDATE above) as Assess-stage approvers." No
-- approval_stage exists yet here on purpose: PatchChangeRequest's own
-- auto-provisioning only creates one the first time {state: "assess"}
-- succeeds (see entity-service's own CLAUDE.md), and this fixture is meant to
-- be driven through that exact transition by the Playwright spec itself.
--
-- requested_by_user_id is deliberately NULL on this and the two fixtures
-- below (not jane.doe/john.smith, both team members) -- confirmed live
-- against real ServiceNow (CHG0039122) and now mirrored by PatchChangeRequest
-- itself (see entity-service's own CLAUDE.md): the change's own requester is
-- auto-provisioned as a Cancelled approver, not Requested, same as real SN.
-- Either seeded user as requester here would silently turn one of these two
-- fixtures' "both members become pending approvers" / "approving one cancels
-- the other pending sibling" demonstrations into a demonstration of self-
-- exclusion instead -- a real, already-covered-elsewhere behavior (see the
-- entity-service integration tests), but not what these three fixtures exist
-- to show.
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id, assignment_group_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000001002', now(), now(), 'seed', 'seed', 'CHG-FIXED-002', 'E2E fixture: New change request with an assigned team', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: New state, assignment_group_id set to the seeded Example Corp ABT group -- Move to Assess must succeed and auto-provision that team''s members as approvers.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO change_request (id, state, priority, impact, category, risk, change_request_type, requested_by_user_id, justification) VALUES
  ('00000000-0000-0000-0000-000000001002', 'NEW'::change_request_state_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: team assigned, used to cover Assess-entry approver auto-provisioning.')
ON CONFLICT (id) DO NOTHING;

-- CR-FIXED-003: ASSESS, assignment_group_id = 901, with its approval_stage +
-- approval_stage_approver rows already created (simulating a CR that already
-- went through auto-provisioning) -- both jane.doe and john.smith sit as
-- "requested" approvers on one stage, covering "both of two approvers see a
-- pending Approve/Reject action; after one approves, the CR cascades to
-- Authorize and the sibling approver's row is cancelled"
-- (DecideChangeRequestApproval's first-responder-wins quorum rule).
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id, assignment_group_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000001003', now(), now(), 'seed', 'seed', 'CHG-FIXED-003', 'E2E fixture: change request in Assess with two pending approvers', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: Assess state with two requested approvers (jane.doe, john.smith) on one stage -- approving one must cascade to Authorize and cancel the other.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO change_request (id, state, priority, impact, category, risk, change_request_type, requested_by_user_id, justification) VALUES
  ('00000000-0000-0000-0000-000000001003', 'ASSESS'::change_request_state_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Assess state, approval already provisioned, used to cover the approve/cancel-sibling cascade.')
ON CONFLICT (id) DO NOTHING;

-- approval_stage/approval_stage_approver mirror what PatchChangeRequest's own
-- auto-provisioning would have written for CR-FIXED-003 on a real
-- {state: "assess"} transition (see entity-service's own CLAUDE.md) --
-- raw_status/status use that same code's lowercase ServiceNow-passthrough
-- vocabulary ("requested", not "REQUESTED"; migration 0089's own doc comment).
INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status) VALUES
  ('00000000-0000-0000-0000-000000001005', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000901', 'requested')
ON CONFLICT (id) DO NOTHING;

INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, status) VALUES
  ('00000000-0000-0000-0000-000000001006', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000001', 'requested'),
  ('00000000-0000-0000-0000-000000001007', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000000002', 'requested')
ON CONFLICT (id) DO NOTHING;

-- CR-FIXED-004: AUTHORIZE, assignment_group_id = 901, with its
-- approval_stage_approver rows already resolved the way #3 should look after
-- a decision -- jane.doe approved, john.smith's sibling row cancelled -- for
-- "an already-decided CR shows the right terminal approval state and offers
-- whatever its legal next states are."
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id, assignment_group_id, opened_by_user_id, assigned_to_id, description) VALUES
  ('00000000-0000-0000-0000-000000001004', now(), now(), 'seed', 'seed', 'CHG-FIXED-004', 'E2E fixture: change request in Authorize with a resolved approval', 'CHANGE_REQUEST', '00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000901', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Seed fixture for Playwright E2E coverage: Authorize state with a resolved approval (jane.doe approved, john.smith cancelled) -- a terminal approval decision already applied.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO change_request (id, state, priority, impact, category, risk, change_request_type, requested_by_user_id, justification) VALUES
  ('00000000-0000-0000-0000-000000001004', 'AUTHORIZE'::change_request_state_enum, 'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum, 'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, NULL, 'Seed fixture: Authorize state, approval already decided, used to cover a terminal-approval-state display.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status) VALUES
  ('00000000-0000-0000-0000-000000001008', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000901', 'approved')
ON CONFLICT (id) DO NOTHING;

INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, status) VALUES
  ('00000000-0000-0000-0000-000000001009', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001008', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000001', 'approved'),
  ('00000000-0000-0000-0000-000000001010', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001008', '00000000-0000-0000-0000-000000001004', '00000000-0000-0000-0000-000000000002', 'cancelled')
ON CONFLICT (id) DO NOTHING;

-- Retrofit for an environment that already seeded CR-FIXED-002/003/004 before
-- requested_by_user_id was nulled out above (ON CONFLICT DO NOTHING on the
-- INSERTs above would never pick up that change on a re-run) -- same
-- always-apply-regardless-of-conflict shape as the account.cre_team_id/
-- team_member.group_id UPDATEs earlier in this file.
UPDATE change_request SET requested_by_user_id = NULL
WHERE id IN ('00000000-0000-0000-0000-000000001002', '00000000-0000-0000-0000-000000001003', '00000000-0000-0000-0000-000000001004');

COMMIT;
