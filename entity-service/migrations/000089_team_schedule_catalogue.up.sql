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

-- The windows CRE and SRE actually work, as they stand today. This is
-- reference data, not sample data: the day view, the week view and the
-- roster all read their shape from here, so an empty catalogue renders an
-- empty page. Minutes are counted from midnight IST; anything past 1440
-- runs into the next day.
--
-- It is seeded as a migration rather than by hand so every environment --
-- local compose, staging, production -- starts from the same catalogue, and
-- so a window that changes is a reviewable diff rather than a console edit.
-- Every statement is idempotent: re-running it changes nothing.

-- ── SRE time zones ────────────────────────────────────────────────────────
INSERT INTO schedule_zone (code, label, sort_order, created_by, updated_by)
VALUES
    ('TZ1', 'Time zone 1', 1, 'migration', 'migration'),
    ('TZ2', 'Time zone 2', 2, 'migration', 'migration'),
    ('TZ3', 'Time zone 3', 3, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

-- At the weekend the three weekday zones collapse into two: TZ1 and TZ2 are
-- both covered by the weekend TZ1 crew (06:00-18:00), and TZ3 by the weekend
-- TZ2 crew (18:00-06:00).
UPDATE schedule_zone z SET weekend_zone_id = w.id, updated_on = NOW()
FROM schedule_zone w
WHERE w.code = CASE z.code WHEN 'TZ1' THEN 'TZ1' WHEN 'TZ2' THEN 'TZ1' WHEN 'TZ3' THEN 'TZ2' END
  AND z.weekend_zone_id IS DISTINCT FROM w.id;

-- ── CRE windows ───────────────────────────────────────────────────────────
INSERT INTO schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, sort_order, created_by, updated_by)
VALUES
    ('CRE_MORNING',    'Morning 6-9am',         'CRE', NULL, NULL, 'WEEKDAY',  360,  540, FALSE, FALSE, 10, 'migration', 'migration'),
    ('CRE_MORNING_OC', 'Morning 6-9am on-call', 'CRE', NULL, NULL, 'WEEKDAY',  360,  540, TRUE,  FALSE, 20, 'migration', 'migration'),
    ('CRE_REGULAR',    'Regular hours',         'CRE', NULL, NULL, 'WEEKDAY',  540, 1080, FALSE, FALSE, 30, 'migration', 'migration'),
    ('CRE_EVENING',    'Evening 6-9pm',         'CRE', NULL, NULL, 'WEEKDAY', 1080, 1260, FALSE, FALSE, 40, 'migration', 'migration'),
    ('CRE_AMERICAS',   'Americas cover',        'CRE', NULL, NULL, 'ANY',     1260, 1800, FALSE, FALSE, 50, 'migration', 'migration'),
    ('CRE_WEEKEND',    'Weekend rotation',      'CRE', NULL, NULL, 'WEEKEND',  540, 1080, FALSE, FALSE, 60, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

-- ── SRE windows ───────────────────────────────────────────────────────────
-- TZ1 and TZ2 run a shorter L1 block than the rest of the zone, so the L1
-- handover lands at 13:30 and 21:00. TZ3 has no separate L1 block at all --
-- every tier works the one 21:00-06:00 stretch -- so it gets a single row
-- with tier NULL rather than an L1 row that would imply a handover that
-- does not happen.
INSERT INTO schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, sort_order, created_by, updated_by)
SELECT v.code, v.label, 'SRE'::schedule_shift_family_enum, z.id, v.tier::schedule_tier_enum,
       v.day_scope::schedule_day_scope_enum, v.start_minute, v.end_minute,
       FALSE, v.is_escalation, v.sort_order, 'migration', 'migration'
FROM (VALUES
    ('SRE_TZ1_L1', 'TZ1 L1 support',     'TZ1', 'L1',  'WEEKDAY',  360,  810, TRUE,  110),
    ('SRE_TZ1',    'TZ1 escalation',     'TZ1', NULL,  'WEEKDAY',  360,  900, TRUE,  120),
    ('SRE_TZ2_L1', 'TZ2 L1 support',     'TZ2', 'L1',  'WEEKDAY',  810, 1260, TRUE,  130),
    ('SRE_TZ2',    'TZ2 escalation',     'TZ2', NULL,  'WEEKDAY',  720, 1260, TRUE,  140),
    ('SRE_TZ3',    'TZ3 escalation',     'TZ3', NULL,  'WEEKDAY', 1260, 1800, TRUE,  150),
    ('SRE_WE_TZ1', 'Weekend TZ1',        'TZ1', NULL,  'WEEKEND',  360, 1080, TRUE,  160),
    ('SRE_WE_TZ2', 'Weekend TZ2',        'TZ2', NULL,  'WEEKEND', 1080, 1800, TRUE,  170)
) AS v(code, label, zone_code, tier, day_scope, start_minute, end_minute, is_escalation, sort_order)
JOIN schedule_zone z ON z.code = v.zone_code
ON CONFLICT (code) DO NOTHING;

-- SRE engineers who are not holding an escalation tier that day work
-- ordinary hours. The zone they belong to rides on the assignment, so this
-- window carries none of its own.
INSERT INTO schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, sort_order, created_by, updated_by)
VALUES
    ('SRE_REGULAR', 'Regular hours', 'SRE', NULL, NULL, 'WEEKDAY', 540, 1080, FALSE, FALSE, 100, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;
