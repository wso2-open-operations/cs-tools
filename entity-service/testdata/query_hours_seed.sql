-- Seed for the query-hour port's end-to-end test. Five projects, one per
-- scenario the thresholds and the push logic have to get right.
--
-- Apply AFTER csm-sync-service's 84 migrations and entity-service's
-- 000084_create_project_query_hours.up.sql. Idempotent: re-running it resets
-- every scenario to its starting state.

BEGIN;

DELETE FROM project_query_hours WHERE project_id IN (
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222',
  '33333333-3333-3333-3333-333333333333',
  '44444444-4444-4444-4444-444444444444',
  '55555555-5555-5555-5555-555555555555');
DELETE FROM time_card WHERE created_by = 'query-hours-seed';
DELETE FROM work_item WHERE created_by = 'query-hours-seed';
DELETE FROM project WHERE created_by = 'query-hours-seed';
DELETE FROM "user" WHERE created_by = 'query-hours-seed';
DELETE FROM account WHERE created_by = 'query-hours-seed';

-- One account, so the ServiceNow fan-out would have swept ALL five projects
-- on any single approval. The port recomputes only the card's own project;
-- sharing the account here is what makes that observable.
INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
VALUES ('aaaaaaaa-0000-0000-0000-000000000001', NOW(), NOW(),
        'query-hours-seed', 'query-hours-seed', 'Seed Account', 'ACCT-SEED', 'SF-ACCT-SEED');

INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, email, first_name, last_name)
VALUES ('bbbbbbbb-0000-0000-0000-000000000001', NOW(), NOW(),
        'query-hours-seed', 'query-hours-seed', 'seed.engineer@wso2.com',
        'seed.engineer@wso2.com', 'Seed', 'Engineer');

-- total_query_duration is an INTERVAL. ServiceNow stored "100h 0m" as text
-- and needed two rival parsers for it; nothing here parses anything.
INSERT INTO project (id, created_on, updated_on, created_by, updated_by,
                     key, sf_id, name, account_id, is_active, total_query_duration)
VALUES
  -- 1. Well under the first threshold: 600 of 6000 minutes = 10% -> state 0.
  ('11111111-1111-1111-1111-111111111111', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'SEED-NORMAL', 'a0d000000000000001', 'Seed Normal',
   'aaaaaaaa-0000-0000-0000-000000000001', TRUE, INTERVAL '100 hours'),
  -- 2. Exactly 75% -> state 1. Boundary is inclusive in ServiceNow.
  ('22222222-2222-2222-2222-222222222222', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'SEED-WARNING', 'a0d000000000000002', 'Seed Warning',
   'aaaaaaaa-0000-0000-0000-000000000001', TRUE, INTERVAL '100 hours'),
  -- 3. Exactly 90% -> state 2.
  ('33333333-3333-3333-3333-333333333333', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'SEED-CRITICAL', 'a0d000000000000003', 'Seed Critical',
   'aaaaaaaa-0000-0000-0000-000000000001', TRUE, INTERVAL '100 hours'),
  -- 4. Overrun: 125% -> state 3, and remainingMinutes must go NEGATIVE.
  ('44444444-4444-4444-4444-444444444444', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'SEED-EXCEEDED', 'a0d000000000000004', 'Seed Exceeded',
   'aaaaaaaa-0000-0000-0000-000000000001', TRUE, INTERVAL '100 hours'),
  -- 5. No entitlement at all. ServiceNow skipped these outright to dodge a
  -- divide-by-zero; the port records 0% rather than omitting the project.
  ('55555555-5555-5555-5555-555555555555', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'SEED-NOQUOTA', 'a0d000000000000005', 'Seed No Quota',
   'aaaaaaaa-0000-0000-0000-000000000001', TRUE, NULL);

-- time_card.case_id is NOT NULL and FKs to work_item, so each project needs one.
INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, account_id, project_id)
SELECT ('cccccccc-0000-0000-0000-00000000000' || n)::uuid, NOW(), NOW(),
       'query-hours-seed', 'query-hours-seed', 'SEED-CASE-' || n, 'Seed case ' || n,
       'aaaaaaaa-0000-0000-0000-000000000001',
       (repeat(n::text, 8) || '-' || repeat(n::text, 4) || '-' || repeat(n::text, 4) || '-'
        || repeat(n::text, 4) || '-' || repeat(n::text, 12))::uuid
FROM generate_series(1, 5) AS n;

-- Minutes are split across the five per-activity columns on purpose: the
-- repository sums all five, so a seed that only filled one would not prove it.
-- ServiceNow summed a single `total` column that Postgres does not have.
INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by,
                       case_id, customer_project_id, user_id, work_date,
                       is_billable, state,
                       analyzing_minutes, setting_up_minutes,
                       reproducing_debugging_minutes, providing_solution_minutes,
                       patching_minutes)
VALUES
  -- 1. 10% of 6000 = 600 billable.
  ('dddddddd-0000-0000-0000-000000000001', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, TRUE, 'APPROVED',
   100, 100, 100, 100, 200),

  -- 2. 75% of 6000 = 4500, split billable 4000 / non-billable 500 so the
  --    split is checked as well as the total.
  ('dddddddd-0000-0000-0000-000000000002', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, TRUE, 'APPROVED',
   1000, 1000, 1000, 1000, 0),
  ('dddddddd-0000-0000-0000-000000000012', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, FALSE, 'APPROVED',
   500, 0, 0, 0, 0),

  -- 3. 90% of 6000 = 5400 billable.
  ('dddddddd-0000-0000-0000-000000000003', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, TRUE, 'APPROVED',
   1400, 1000, 1000, 1000, 1000),

  -- 4. 125% of 6000 = 7500 billable.
  ('dddddddd-0000-0000-0000-000000000004', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000004', '44444444-4444-4444-4444-444444444444',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, TRUE, 'APPROVED',
   1500, 1500, 1500, 1500, 1500),

  -- 5. Consumption with no entitlement.
  ('dddddddd-0000-0000-0000-000000000005', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000005', '55555555-5555-5555-5555-555555555555',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, TRUE, 'APPROVED',
   300, 0, 0, 0, 0),

  -- 6. A SUBMITTED card on project 1. Must NOT count: only APPROVED does.
  --    Deliberately large, so if the state filter regresses the test screams.
  ('dddddddd-0000-0000-0000-000000000006', NOW(), NOW(), 'query-hours-seed', 'query-hours-seed',
   'cccccccc-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111',
   'bbbbbbbb-0000-0000-0000-000000000001', CURRENT_DATE, TRUE, 'SUBMITTED',
   9000, 0, 0, 0, 0);

COMMIT;
