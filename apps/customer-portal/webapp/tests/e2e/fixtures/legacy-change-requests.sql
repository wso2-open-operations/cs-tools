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

-- LEGACY change requests, shaped like the ones migrated or synced from ServiceNow, for the
-- customer portal's real-stack specs (customer-change-request-legacy.spec.ts). LOCAL STACK ONLY:
-- applied with `psql` by the spec itself (utils/localStack.ts: seedLegacyChangeRequests), never by
-- the stack's own seed, because a migrated row is exactly what the local seed cannot be.
--
-- What makes a row "migrated" here (entity-service/CLAUDE.md, "Customer visibility and the cutover"):
--   * created BEFORE the cutover instant. The local compose stack runs entity-service with
--     CR_STRICT_VISIBILITY_FROM=2000-01-01T00:00:00Z (docker-compose.yml), so "before" is
--     1999-12-20: no native row can be that old, and nothing else decides legacy-ness (no column
--     says where a change request came from);
--   * a ServiceNow-style id (a sys_id rendered as a UUID: random hex, not the seed's 0000...
--     pattern) and number (CHG00391NN), created_by 'sn-sync';
--   * NO customer-stage rows (approval_stage labelled 'Customer Approval' / 'Customer Review'
--     with customer approver rows) and customer_approval_required / customer_review_required =
--     false: a migrated change request never passed through this portal's approval flow;
--   * approval stages the sync mirrored have a NULL checkpoint_label (CHG0039301/-302 below), an
--     approver whose user row is missing in one case, and are decided by position / group alone.
--
-- Idempotent and self-healing: every row below is deleted and re-inserted, so running the file
-- again puts them back to this starting state (a spec that answers one re-runs it first).
--
-- The ids are md5(number) rendered as a UUID, which is how utils/localStack.ts (legacyId) finds them.
--
-- Rows on "Example Corp Production" (project 401: dave, erin), one per lifecycle state, flags false,
-- nothing asked of anybody:
--   CHG0039101 New               CHG0039107 Review
--   CHG0039102 Assess            CHG0039108 Customer Review   (no live stage: asked before this build)
--   CHG0039103 Authorize         CHG0039109 Rollback
--   CHG0039104 Customer Approval (no live stage: asked before this build)
--   CHG0039105 Scheduled         CHG0039110 Closed            (outcome stamps true: SN's own meaning)
--   CHG0039106 Implement         CHG0039111 Canceled
--   CHG0039112 Customer Approval, a second one for the proposal scenario (a window already planned)
--   CHG0039113 Customer Approval, a third one for the second contact's answer (erin)
-- Rows on "Lumen Works Platform" (mira, noel; found by name, the id is random per database):
--   CHG0039201 Customer Approval  (no live stage: the user's "Demo Test 1")
--   CHG0039202 Scheduled
-- Synced-stage shapes on project 401:
--   CHG0039301 Emergency, Authorize, ONE stage with no label at position 0 (alice and bob REQUESTED)
--   CHG0039302 Normal, Scheduled, a stale stage with no label at position 0 (bob REQUESTED, and a
--              row whose user is missing)
-- Either side of the cutover (both Scheduled, nothing asked, on project 401):
--   CHG0039401 created 1999-12-31 23:59:59Z  -> legacy: customers see it
--   CHG0039402 created 2000-01-01 00:00:00Z  -> strict (at the instant is NOT before it): customers do not
--
-- Run as the postgres superuser (the specs' psql helper); the rows are written the way csm-sync-service
-- writes them, not through entity-service.

BEGIN;

CREATE TEMP TABLE legacy_cr (number text, state text, model text, subject text, project_name text,
                             created_on timestamptz, stamp_a boolean, stamp_r boolean, planned boolean)
  ON COMMIT DROP;
INSERT INTO legacy_cr VALUES
  ('CHG0039101','NEW',               'NORMAL',   'Legacy: New',                                                'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039102','ASSESS',            'NORMAL',   'Legacy: Assess',                                             'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039103','AUTHORIZE',         'NORMAL',   'Legacy: Authorize',                                          'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039104','CUSTOMER_APPROVAL', 'NORMAL',   'Legacy: Customer Approval with no live stage',               'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039105','SCHEDULED',         'NORMAL',   'Legacy: Scheduled',                                          'Example Corp Production', '1999-12-20 09:00:00+00', false, false, false),
  ('CHG0039106','IMPLEMENT',         'NORMAL',   'Legacy: Implement',                                          'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039107','REVIEW',            'NORMAL',   'Legacy: Review',                                             'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039108','CUSTOMER_REVIEW',   'NORMAL',   'Legacy: Customer Review with no live stage',                 'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039109','ROLLBACK',          'NORMAL',   'Legacy: Rollback',                                           'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039110','CLOSED',            'NORMAL',   'Legacy: Closed',                                             'Example Corp Production', '1999-12-20 09:00:00+00', true,  true,  false),
  ('CHG0039111','CANCELED',          'NORMAL',   'Legacy: Canceled',                                           'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039112','CUSTOMER_APPROVAL', 'NORMAL',   'Legacy: Customer Approval to propose a time on',             'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, true),
  ('CHG0039113','CUSTOMER_APPROVAL', 'NORMAL',   'Legacy: Customer Approval for the second contact',           'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039201','CUSTOMER_APPROVAL', 'NORMAL',   'Demo Test 1 (legacy, Customer Approval, no live stage)',     'Lumen Works Platform',    '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039202','SCHEDULED',         'NORMAL',   'Legacy: Scheduled on Lumen',                                 'Lumen Works Platform',    '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039301','AUTHORIZE',         'EMERGENCY','Legacy: Emergency in Authorize, synced stage with no label','Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039302','SCHEDULED',         'NORMAL',   'Legacy: Scheduled, stale synced stage with no label',        'Example Corp Production', '1999-12-20 09:00:00+00', NULL, NULL, false),
  ('CHG0039401','SCHEDULED',         'NORMAL',   'Boundary: created one second before the cutover',            'Example Corp Production', '1999-12-31 23:59:59+00', NULL, NULL, false),
  ('CHG0039402','SCHEDULED',         'NORMAL',   'Boundary: created at the cutover instant',                   'Example Corp Production', '2000-01-01 00:00:00+00', NULL, NULL, false);

-- put the previous run's rows away (the cascade takes the change_request, stage and approver rows)
DELETE FROM work_item WHERE number IN (SELECT number FROM legacy_cr);

INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id,
                       assignment_group_id, opened_by_user_id, assigned_to_id, description)
SELECT md5('legacy-' || l.number)::uuid, l.created_on, l.created_on, 'sn-sync', 'sn-sync', l.number, l.subject, 'CHANGE_REQUEST',
       p.account_id, p.id, NULL, NULL, NULL,
       'Migrated from ServiceNow: no customer-stage rows, customer approval / review not required, created before the strict-visibility cutover.'
FROM legacy_cr l
JOIN LATERAL (SELECT id, account_id FROM project WHERE name = l.project_name AND account_id IS NOT NULL ORDER BY created_on, id LIMIT 1) p ON true;

INSERT INTO change_request (id, state, change_model, priority, impact, category, risk, change_request_type, justification,
                            customer_approval_required, customer_review_required, is_customer_approval_required, is_customer_review_required,
                            start_on, end_on)
SELECT md5('legacy-' || l.number)::uuid, l.state::change_request_state_enum, l.model::change_request_change_model_enum,
       'MODERATE'::change_request_priority_enum, 'LOW'::change_request_impact_enum, 'SOFTWARE'::change_request_category_enum,
       'LOW'::change_request_risk_enum, 'GENERAL'::change_request_type_enum, 'Migrated from ServiceNow.',
       false, false, l.stamp_a, l.stamp_r,
       CASE WHEN l.planned THEN '2031-03-01 10:00:00+00'::timestamptz END,
       CASE WHEN l.planned THEN '2031-03-01 12:00:00+00'::timestamptz END
FROM legacy_cr l
WHERE EXISTS (SELECT 1 FROM work_item w WHERE w.id = md5('legacy-' || l.number)::uuid);

-- The synced stages: NO checkpoint_label, no assignment group (an unsynced ServiceNow group also yields NULL).
-- CHG0039301: the single stage of an Emergency change in Authorize (the ECAB's, read from state and type).
-- CHG0039302: a stale stage of a Normal change that has moved on to Scheduled.
INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label)
SELECT md5('legacy-stage-' || n)::uuid, '1999-12-20 09:30:00+00', '1999-12-20 09:30:00+00', 'sn-sync', 'sn-sync',
       md5('legacy-' || n)::uuid, NULL, 'requested', NULL
FROM (VALUES ('CHG0039301'), ('CHG0039302')) v(n)
WHERE EXISTS (SELECT 1 FROM work_item w WHERE w.id = md5('legacy-' || n)::uuid);

INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
SELECT md5('legacy-approver-' || a.n || a.who)::uuid, '1999-12-20 09:30:00+00', '1999-12-20 09:30:00+00', 'sn-sync', 'sn-sync',
       md5('legacy-stage-' || a.n)::uuid, md5('legacy-' || a.n)::uuid, a.user_id::uuid, 'REQUESTED'
FROM (VALUES
  ('CHG0039301', 'alice', '00000000-0000-0000-0000-000000000011'),
  ('CHG0039301', 'bob',   '00000000-0000-0000-0000-000000000012'),
  ('CHG0039302', 'bob',   '00000000-0000-0000-0000-000000000012'),
  ('CHG0039302', 'nobody', NULL)
) a(n, who, user_id)
WHERE EXISTS (SELECT 1 FROM approval_stage s WHERE s.id = md5('legacy-stage-' || a.n)::uuid);

COMMIT;
