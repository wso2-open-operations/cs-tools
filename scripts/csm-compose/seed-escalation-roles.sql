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

-- ASSUMED DATA. Delete this file once the real answers arrive.
--
-- The call-escalation ladder needs five rungs' worth of people, and two of
-- them have no source yet:
--
--   LEVEL_0  the sub lead rostered on the covering rotation
--   LEVEL_1  the ABT's three sub leads
--   LEVEL_2  that ABT's one lead          <- import-abt-roster.py already has this
--   LEVEL_3  CRE head
--   LEVEL_4  CS head
--
-- Who the three sub leads are is still to be decided, and the ABT roster sheet
-- carries no column for them; the two heads are not in any roster at all. This
-- file invents both so the ladder can be exercised end to end against a local
-- stack. Nothing here is real, and none of it should outlive the decision.
--
-- Runs after seed-team-schedule.sql, which creates the teams and members this
-- promotes. Idempotent: re-running promotes nobody new and inserts nothing
-- twice.

BEGIN;

-- Three sub leads per CRE ABT, chosen by e-mail so a rebuild picks the same
-- people. Tops each team up to three rather than promoting three more every
-- run, which is what makes this safe to re-apply.
WITH shortfall AS (
    SELECT t.id AS team_id,
           3 - count(*) FILTER (WHERE m.role = 'sub_lead') AS wanted
      FROM team t
      LEFT JOIN team_member m ON m.team_id = t.id
     WHERE t.type = 'cre-abt'
     GROUP BY t.id
), candidates AS (
    SELECT m.id, m.team_id,
           row_number() OVER (PARTITION BY m.team_id ORDER BY u.email) AS rn
      FROM team_member m
      JOIN "user" u ON u.id = m.user_id
     WHERE m.role = 'engineer'
)
UPDATE team_member m
   SET role = 'sub_lead', updated_on = now(), updated_by = 'seed:assumed-roles'
  FROM candidates c
  JOIN shortfall s ON s.team_id = c.team_id
 WHERE m.id = c.id
   AND s.wanted > 0
   AND c.rn <= s.wanted;

-- LEVEL_0 is whoever is rostered on the covering rotation AND is a sub lead,
-- so a shift family with no sub lead on any of its windows cannot exercise the
-- rung at all. The generated rota fills slots round robin from the whole team,
-- so whether a sub lead lands on a given window is chance; CRE_WEEKEND_NIGHT,
-- with only a handful of slots, comes up empty. Promote one rostered engineer
-- for each family that has none, so every family is reachable by a test.
--
-- This deliberately does NOT reproduce the real rota's guarantee, which is
-- that a sub lead is scheduled onto every rotation. Reaching that by promotion
-- would mean promoting most of the team -- three sub leads spread over a round
-- robin of sixteen will miss most windows -- and that would destroy LEVEL_1,
-- where "the ABT's three sub leads" is the entire point. The real guarantee
-- comes from how the rota is built, not from who holds which role.
WITH uncovered AS (
    SELECT s.id AS shift_id
      FROM team_schedule_shift s
     WHERE s.family = 'CRE'
       AND NOT EXISTS (
           SELECT 1
             FROM team_schedule_assignment a
             JOIN team_member m ON m.user_id = a.user_id AND m.role = 'sub_lead'
            WHERE a.shift_id = s.id)
), pick AS (
    SELECT DISTINCT ON (a.shift_id) a.shift_id, m.id AS membership_id
      FROM team_schedule_assignment a
      JOIN uncovered u ON u.shift_id = a.shift_id
      JOIN team_member m ON m.user_id = a.user_id AND m.role = 'engineer'
     ORDER BY a.shift_id, m.id
)
UPDATE team_member m
   SET role = 'sub_lead', updated_on = now(), updated_by = 'seed:assumed-roles'
  FROM pick p
 WHERE m.id = p.membership_id;

-- The heads sit in their own team, not an ABT. That is deliberate: a head must
-- still resolve to no ABT for /users/me, which is the absence the Team Schedule
-- page reads as "belongs to neither group".
INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
VALUES (md5('seed-team-cre-leadership')::uuid, now(), now(), 'seed', 'seed',
        'CRE Leadership', 'cre-leadership', 'cre-leadership')
ON CONFLICT (id) DO NOTHING;

INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
                    user_name, name, first_name, last_name, email, is_active, is_system_user)
VALUES
  (md5('seed-cre-head')::uuid, now(), now(), 'seed', 'seed',
   'cre.head@example.com', 'CRE Head', 'CRE', 'Head', 'cre.head@example.com', TRUE, FALSE),
  (md5('seed-cs-head')::uuid, now(), now(), 'seed', 'seed',
   'cs.head@example.com', 'CS Head', 'CS', 'Head', 'cs.head@example.com', TRUE, FALSE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
VALUES
  (md5('seed-tm-cre-head')::uuid, now(), now(), 'seed', 'seed',
   md5('seed-team-cre-leadership')::uuid, md5('seed-cre-head')::uuid, 'cre_head'),
  (md5('seed-tm-cs-head')::uuid, now(), now(), 'seed', 'seed',
   md5('seed-team-cre-leadership')::uuid, md5('seed-cs-head')::uuid, 'cs_head')
ON CONFLICT (id) DO NOTHING;

-- Alert-duty nominees (T1/T2/T3) for every ABT team.
--
-- LEVEL_0 on the business-hours rules IS these people: R2 calls the incident's
-- own ABT's three, and R3 -- an incident on no ABT -- calls one from each ABT,
-- which is seven calls. Without them that rung resolves to nobody, so the
-- fastest rung of a business-hours ladder silently does not happen, and a
-- local run looks like it is working because the rungs above still fire.
--
-- Three per team, lowest user_id first, so a volume reset reproduces the same
-- nominees and a test asserting who gets called stays stable. Deterministic,
-- not meaningful: these are seeded synthetic people.
-- Cleared first: alert_tier carries a UNIQUE (team_id, alert_tier) index, so
-- re-seeding a team whose T1 is currently somebody else collides rather than
-- moving the nomination. Clearing inside the same transaction makes this file
-- re-runnable, which migrate-and-seed.sh relies on -- it runs every time.
UPDATE team_member m
   SET alert_tier = NULL, updated_on = now(), updated_by = 'seed:alert-duty'
  FROM team t
 WHERE t.id = m.team_id
   AND t.type IN ('cre-abt', 'sre-abt')
   AND m.alert_tier IS NOT NULL;

WITH ranked AS (
  SELECT tm.id,
         ROW_NUMBER() OVER (PARTITION BY tm.team_id ORDER BY tm.user_id) AS rn
    FROM team_member tm
    JOIN team t ON t.id = tm.team_id
   WHERE t.type IN ('cre-abt', 'sre-abt')
     AND tm.role = 'engineer'
)
UPDATE team_member m
   SET alert_tier = 'T' || r.rn, updated_on = now(), updated_by = 'seed:alert-duty'
  FROM ranked r
 WHERE m.id = r.id
   AND r.rn <= 3;

COMMIT;
