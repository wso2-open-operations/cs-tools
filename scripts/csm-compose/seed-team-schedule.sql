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

-- LOCAL DEVELOPMENT ONLY. Dummy Team Schedule data: the CRE and SRE teams,
-- their engineers, and a rota either side of today so every view has
-- something to show.
--
-- This is a seed, not the allocator. It fills the shape the UI reads, by
-- round-robin over a stable ordering:
--
--   CRE weekday   one morning 6-9am and one morning on-call, from any ABT;
--                 evening 6-9pm from every ABT, so all seven are on it
--   CRE weekend   three engineers from any ABT, plus one from Americas
--   Americas      the whole team on night cover each weekday -- that is when
--                 they work, not a rotation they take turns at
--   SRE weekday   two L1 and two L2 per zone, one of each from both SRE teams
--
-- The real allocator, with leave awareness and swap handling, belongs in
-- entity-service.
--
-- Ids are derived with md5() so re-running changes nothing and a given
-- engineer keeps the same id across rebuilds.
--
-- Everyone not holding a rotation gets a regular-hours row rather than being
-- inferred by subtraction: team_member carries no validity dates, so a
-- derived view of a past day would silently apply today's membership to it.

BEGIN;

-- ── teams ─────────────────────────────────────────────────────────────────
-- headcounts mirror the ABT rota plan the prototype was drawn from
CREATE TEMP TABLE _team (key TEXT, display TEXT, family TEXT, headcount INT, ord INT) ON COMMIT DROP;
INSERT INTO _team VALUES
    ('castor',   'Castor',    'cre-abt', 15,  1),
    ('draco',    'Draco',     'cre-abt', 16,  2),
    ('vega',     'Vega',      'cre-abt', 20,  3),
    ('sirius',   'Sirius',    'cre-abt', 12,  4),
    ('atlas',    'Atlas',     'cre-abt', 12,  5),
    ('phoenix',  'Phoenix',   'cre-abt', 14,  6),
    ('rigel',    'Rigel',     'cre-abt', 11,  7),
    ('americas', 'Americas',  'cre',     12,  8),
    ('migration','Migration', 'cre',     10,  9),
    ('apollo',   'Apollo',    'sre-abt', 18, 10),
    ('artemis',  'Artemis',   'sre-abt', 18, 11);

INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type)
SELECT md5('seed-team-'||key)::uuid, now(), now(), 'seed', 'seed', display, family
FROM _team
ON CONFLICT (id) DO NOTHING;

-- ── engineers ─────────────────────────────────────────────────────────────
CREATE TEMP TABLE _eng (id UUID, team_key TEXT, family TEXT, seq INT, is_lead BOOLEAN) ON COMMIT DROP;
INSERT INTO _eng
SELECT md5('seed-eng-'||t.key||'-'||g)::uuid, t.key, t.family, g, (g = 1)
FROM _team t, generate_series(1, t.headcount) g;

INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
                    user_name, name, first_name, last_name, email, is_active, is_system_user)
SELECT e.id, now(), now(), 'seed', 'seed',
       e.team_key||'.'||lpad(e.seq::text,2,'0')||'@example.com',
       t.display||' Engineer '||lpad(e.seq::text,2,'0'),
       t.display, 'Engineer '||lpad(e.seq::text,2,'0'),
       e.team_key||'.'||lpad(e.seq::text,2,'0')||'@example.com', TRUE, FALSE
FROM _eng e JOIN _team t ON t.key = e.team_key
ON CONFLICT (id) DO NOTHING;

-- ── a manager ─────────────────────────────────────────────────────────────
-- Deliberately gets a user row and NO team_member row. That absence is how a
-- manager is recognised: they sit above the ABTs rather than in one, so
-- /users/me resolves no team for them and the page reads that as "belongs to
-- neither group" rather than having to carry a role flag of its own.
--
-- Sign in as manager@example.com (groups: cs_engineer) to exercise the view.
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
                    user_name, name, first_name, last_name, email, is_active, is_system_user)
VALUES (md5('seed-manager-1')::uuid, now(), now(), 'seed', 'seed',
        'manager@example.com', 'Morgan Manager', 'Morgan', 'Manager',
        'manager@example.com', TRUE, FALSE)
ON CONFLICT (id) DO NOTHING;

-- The engineers and the manager are WSO2 staff, so give them the internal role.
--
-- Not cosmetic: recompute_user_type() derives "user".user_type from a user's
-- roles, and the schedule endpoints admit only an unrestricted caller, which
-- ResolveScope grants only to an INTERNAL user. Without this every engineer is
-- NOT_AVAILABLE and the rota answers 403 to the people it is for.
--
-- Scoped to the rows THIS seed creates -- _eng and the manager -- and not to
-- created_by = 'seed', which was the first attempt and was wrong. That also
-- matches seed-entity-service.sql's users, one of whom is a customer. Granting
-- internal on top of their customer role flips user_type to INTERNAL, because
-- recompute_user_type() checks internal before external, and scopeForUser then
-- hands that customer unrestricted scope over every project in the database.
-- Raised in review; the blast radius is local-dev only, but the predicate was
-- indefensible either way.
--
-- The delete first makes it self-healing: a volume that already took the wrong
-- grant loses it on the next run rather than keeping it forever.
DELETE FROM user_role
 WHERE created_by = 'seed'
   AND role_id = '00000000-0000-0000-0000-000000000101'::uuid
   AND user_id NOT IN (SELECT id FROM _eng UNION ALL SELECT md5('seed-manager-1')::uuid);

INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
SELECT md5('seed-ur-'||u.id::text)::uuid, now(), now(), 'seed', 'seed',
       u.id, '00000000-0000-0000-0000-000000000101'::uuid
FROM (SELECT id FROM _eng UNION ALL SELECT md5('seed-manager-1')::uuid) u
WHERE NOT EXISTS (SELECT 1 FROM user_role ur
                   WHERE ur.user_id = u.id
                     AND ur.role_id = '00000000-0000-0000-0000-000000000101'::uuid)
ON CONFLICT (id) DO NOTHING;

INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
SELECT md5('seed-tm-'||e.team_key||'-'||e.seq)::uuid, now(), now(), 'seed', 'seed',
       md5('seed-team-'||e.team_key)::uuid, e.id,
       CASE WHEN e.is_lead THEN 'lead' ELSE 'member' END
FROM _eng e
ON CONFLICT (id) DO NOTHING;

-- ── the window we roster ──────────────────────────────────────────────────
-- three weeks back and three weeks forward, so "my week", "this week" and
-- "next rotation" all have data whenever the stack is brought up -- widened to
-- the whole calendar month when that reaches further, so the month roster has
-- no blank days at either end. n counts from the first day seeded, so it is
-- never negative and the rotation modulos below stay in range.
CREATE TEMP TABLE _day (d DATE, dow INT, is_weekend BOOLEAN, n INT) ON COMMIT DROP;
INSERT INTO _day
SELECT g::date, EXTRACT(isodow FROM g)::int, EXTRACT(isodow FROM g)::int > 5,
       (g::date - w.lo)::int
FROM (SELECT LEAST(CURRENT_DATE - 21, date_trunc('month', CURRENT_DATE)::date) AS lo,
             GREATEST(CURRENT_DATE + 21,
                      (date_trunc('month', CURRENT_DATE) + interval '1 month - 1 day')::date) AS hi) w,
     generate_series(w.lo, w.hi, interval '1 day') g;

DELETE FROM schedule_assignment WHERE created_by = 'seed';
DELETE FROM schedule_absence   WHERE created_by = 'seed';

-- ── standing allocations, and who that leaves on the rota ────────────────
-- Seeded ahead of the rotas, because they decide who the rotas draw from. The
-- two groups do not draw from the same list.
-- CRE carries migration work; SRE does not -- an SRE engineer is either on
-- R&D or sitting with a customer, on site or off. Seeding migration against
-- SRE would put a category in their off-rota column that does not exist for
-- them.
INSERT INTO schedule_absence (user_id, team_key, kind_id, starts_on, ends_on, note, created_by, updated_by)
SELECT e.id, e.team_key, k.id, CURRENT_DATE - 60, NULL, 'standing allocation', 'seed', 'seed'
FROM _eng e
JOIN schedule_absence_kind k
  ON k.code = CASE
       WHEN e.family = 'sre-abt' THEN
         CASE e.seq WHEN 5 THEN 'RND'
                    WHEN 6 THEN 'CUSTOMER_ONSITE'
                    ELSE 'CUSTOMER_OFFSITE' END
       ELSE
         CASE e.seq WHEN 5 THEN 'RND'
                    WHEN 6 THEN 'CUSTOMER'
                    ELSE 'MIGRATION' END
     END
WHERE e.seq IN (5, 6, 7);

-- An engineer on a standing allocation is doing that work instead, so they
-- are left out of every pool the rotas and regular hours draw from below --
-- otherwise the same person reads as both off-rota and on shift. Leave is
-- different: it is a few days, and the rotas simply run past it.
CREATE TEMP TABLE _on_rota ON COMMIT DROP AS
SELECT e.* FROM _eng e
WHERE NOT EXISTS (
    SELECT 1 FROM schedule_absence ab
    JOIN schedule_absence_kind k ON k.id = ab.kind_id
    WHERE ab.user_id = e.id AND ab.created_by = 'seed' AND k.bucket = 'ALLOCATION');

-- resolve a window on a date, in the clock it was authored in
CREATE OR REPLACE FUNCTION _seed_span(p_day DATE, p_code TEXT)
RETURNS TABLE (shift_id UUID, zone_id UUID, starts_at TIMESTAMPTZ, ends_at TIMESTAMPTZ)
LANGUAGE sql STABLE AS $$
  SELECT s.id, s.zone_id,
         (p_day::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
         (p_day::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone
  FROM schedule_shift s WHERE s.code = p_code;
$$;

-- ── CRE: the ABT rotations ────────────────────────────────────────────────
-- Two pools over the same engineers, because the two rotations pick
-- differently. The morning slots want one person from the ABTs collectively,
-- so they rotate through a single flat order. The evening wants one person
-- from each ABT, so it rotates within each team independently -- which is why
-- every engineer carries both a global rank and a rank among their own team.
CREATE TEMP TABLE _abt (
  id UUID, team_key TEXT,
  rn INT, total INT,          -- position in the flat, all-ABT order
  trn INT, ttotal INT         -- position within this engineer's own team
) ON COMMIT DROP;
INSERT INTO _abt
SELECT e.id, e.team_key,
       (row_number() OVER (ORDER BY t.ord, e.seq))::int,
       (count(*) OVER ())::int,
       (row_number() OVER (PARTITION BY e.team_key ORDER BY e.seq))::int,
       (count(*) OVER (PARTITION BY e.team_key))::int
FROM _on_rota e JOIN _team t ON t.key = e.team_key
WHERE e.family = 'cre-abt';

-- Morning 6-9am and the morning on-call: one engineer each, from any ABT.
INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT a.id, md5('seed-team-'||a.team_key)::uuid, a.team_key, sp.shift_id, sp.zone_id, NULL,
       d.d, sp.starts_at, sp.ends_at, v.code = 'CRE_MORNING_OC', 'GENERATED', 'seed', 'seed'
FROM _day d
CROSS JOIN (VALUES ('CRE_MORNING',0),('CRE_MORNING_OC',1)) AS v(code, slot)
JOIN _abt a ON a.rn = ((d.n * 2 + v.slot) % a.total) + 1
CROSS JOIN LATERAL _seed_span(d.d, v.code) sp
WHERE NOT d.is_weekend
ON CONFLICT DO NOTHING;

-- Evening 6-9pm: one engineer from every ABT, so all seven are represented
-- each weekday. Rotating within the team rather than across the whole pool is
-- what guarantees that -- a flat pool of 100 would happily draw two from
-- Vega and none from Rigel.
INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT a.id, md5('seed-team-'||a.team_key)::uuid, a.team_key, sp.shift_id, sp.zone_id, NULL,
       d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
JOIN _abt a ON a.trn = (d.n % a.ttotal) + 1
CROSS JOIN LATERAL _seed_span(d.d, 'CRE_EVENING') sp
WHERE NOT d.is_weekend
ON CONFLICT DO NOTHING;

-- The weekend rotation: three ABT engineers a day, deliberately from
-- different ABTs.
--
-- The flat order runs team by team -- the first fifteen ranks are all Castor,
-- the next sixteen all Draco -- so taking three *consecutive* ranks, as this
-- did, put three people from the same team on almost every weekend. Spacing
-- the three picks a third of the pool apart lands them in different teams,
-- and stepping seven ranks a day -- a stride that shares no factor with the
-- team sizes -- keeps the trio of teams itself changing from one weekend to
-- the next rather than settling on the same three.
INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT a.id, md5('seed-team-'||a.team_key)::uuid, a.team_key, sp.shift_id, sp.zone_id, NULL,
       d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
CROSS JOIN generate_series(0,2) AS slot
-- Keyed on the Saturday, not on the day: whoever takes a weekend takes both
-- days of it, so Sunday resolves to the same three engineers Saturday did.
-- `dow` is isodow, so Saturday is 6 and Sunday 7 -- subtracting (dow - 6)
-- walks a Sunday back onto its own Saturday.
JOIN _abt a ON a.rn = (((d.n - (d.dow - 6)) * 7 + slot * (a.total / 3)) % a.total) + 1
CROSS JOIN LATERAL _seed_span(d.d, 'CRE_WEEKEND') sp
WHERE d.is_weekend
ON CONFLICT DO NOTHING;

-- Americas cover the night. On a weekday that is not a rota at all -- it is
-- simply when the team works, so the whole team is on it. At the weekend it
-- becomes a rota like the others, and one engineer takes it.
INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT e.id, md5('seed-team-americas')::uuid, 'americas', sp.shift_id, sp.zone_id, NULL,
       d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
JOIN _on_rota e ON e.team_key = 'americas'
CROSS JOIN LATERAL _seed_span(d.d, 'CRE_AMERICAS') sp
WHERE NOT d.is_weekend
ON CONFLICT DO NOTHING;

-- the weekend Americas rota: one engineer, rotating through the team
CREATE TEMP TABLE _ame (id UUID, rn INT, total INT) ON COMMIT DROP;
INSERT INTO _ame
SELECT e.id,
       (row_number() OVER (ORDER BY e.seq))::int,
       (count(*) OVER ())::int
FROM _on_rota e WHERE e.team_key = 'americas';

INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT a.id, md5('seed-team-americas')::uuid, 'americas', sp.shift_id, sp.zone_id, NULL,
       d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
JOIN _ame a ON a.rn = (d.n % a.total) + 1
CROSS JOIN LATERAL _seed_span(d.d, 'CRE_AMERICAS') sp
WHERE d.is_weekend
ON CONFLICT DO NOTHING;

-- ── SRE: two L1 and two L2 per zone per weekday, one of each per team ─────
CREATE TEMP TABLE _sre (id UUID, team_key TEXT, rn INT, total INT) ON COMMIT DROP;
INSERT INTO _sre
SELECT e.id, e.team_key,
       (row_number() OVER (PARTITION BY e.team_key ORDER BY e.seq))::int,
       (count(*) OVER (PARTITION BY e.team_key))::int
FROM _on_rota e WHERE e.family = 'sre-abt';

INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT s.id, md5('seed-team-'||s.team_key)::uuid, s.team_key, sp.shift_id, sp.zone_id,
       v.tier::schedule_tier_enum, d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
CROSS JOIN (VALUES
      ('SRE_TZ1_L1','L1',0), ('SRE_TZ1','L2',1),
      ('SRE_TZ2_L1','L1',2), ('SRE_TZ2','L2',3),
      ('SRE_TZ3',   'L1',4), ('SRE_TZ3','L2',5)
  ) AS v(code, tier, slot)
CROSS JOIN (VALUES ('apollo'),('artemis')) AS tm(team_key)
JOIN _sre s ON s.team_key = tm.team_key AND s.rn = ((d.n * 6 + v.slot) % s.total) + 1
CROSS JOIN LATERAL _seed_span(d.d, v.code) sp
WHERE NOT d.is_weekend
ON CONFLICT DO NOTHING;

-- the weekend runs two zones, one crew each
INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT s.id, md5('seed-team-'||s.team_key)::uuid, s.team_key, sp.shift_id, sp.zone_id,
       'L1'::schedule_tier_enum, d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
CROSS JOIN (VALUES ('SRE_WE_TZ1',0),('SRE_WE_TZ2',1)) AS v(code, slot)
CROSS JOIN (VALUES ('apollo'),('artemis')) AS tm(team_key)
JOIN _sre s ON s.team_key = tm.team_key AND s.rn = ((d.n * 2 + v.slot) % s.total) + 1
CROSS JOIN LATERAL _seed_span(d.d, v.code) sp
WHERE d.is_weekend
ON CONFLICT DO NOTHING;

-- ── leave ─────────────────────────────────────────────────────────────────
-- a few ranges; standing allocations were seeded above, before the rotas
INSERT INTO schedule_absence (user_id, team_key, kind_id, starts_on, ends_on, note, created_by, updated_by)
SELECT e.id, e.team_key, k.id,
       CURRENT_DATE + ((e.seq % 11) - 4), CURRENT_DATE + ((e.seq % 11) - 4) + 3,
       'seeded leave', 'seed', 'seed'
FROM _eng e
JOIN schedule_absence_kind k ON k.code = CASE WHEN e.seq % 2 = 0 THEN 'ANNUAL_LEAVE' ELSE 'LIEU_LEAVE' END
WHERE e.seq IN (4, 9);

-- ── everyone else works regular hours ─────────────────────────────────────
-- stored, not derived: see the note at the top
INSERT INTO schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, created_by, updated_by)
SELECT e.id, md5('seed-team-'||e.team_key)::uuid, e.team_key, sp.shift_id,
       -- SRE regular hours carry a zone of their own: the window has none (every
       -- zone keeps the same 09:00-18:00), but the engineer still belongs to one
       -- that day, and that is what the "others in TZ" card groups by
       CASE WHEN e.family = 'sre-abt' THEN z.id ELSE sp.zone_id END, NULL,
       d.d, sp.starts_at, sp.ends_at, FALSE, 'GENERATED', 'seed', 'seed'
FROM _day d
JOIN _on_rota e ON TRUE
CROSS JOIN LATERAL _seed_span(d.d, CASE WHEN e.family = 'sre-abt' THEN 'SRE_REGULAR' ELSE 'CRE_REGULAR' END) sp
LEFT JOIN LATERAL (
    SELECT id FROM schedule_zone
    WHERE e.family = 'sre-abt'
    ORDER BY sort_order OFFSET ((e.seq + d.n) % 3) LIMIT 1
) z ON TRUE
WHERE NOT d.is_weekend
  AND e.team_key <> 'americas'
  AND NOT EXISTS (SELECT 1 FROM schedule_assignment a
                   WHERE a.user_id = e.id AND a.rota_date = d.d AND a.created_by = 'seed')
ON CONFLICT DO NOTHING;

DROP FUNCTION IF EXISTS _seed_span(DATE, TEXT);

COMMIT;
