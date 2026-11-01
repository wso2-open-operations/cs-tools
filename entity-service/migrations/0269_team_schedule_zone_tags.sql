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

-- Team Schedule: CRE chips name the time zone, and an AEST window.
--
-- 1. The CRE chips named places (LK, IND, NLK). They now name the clock the
--    hours are worked on:
--      LK  (Regular hours)            -> IST
--      IND (India region shift)       -> IST
--      NLK (Americas cover / regular) -> PDT
--      NLK-WE    (Americas weekend)          -> PDT-WE
--      NLK-WE-OC (Americas weekend on-call)  -> PDT-WE-OC
--    A fixed label: it reads PDT all year, also while Los Angeles is on PST.
--    Only short_code changes. Rows are keyed by code and coloured by
--    colour_token, which stay as they are, so nothing that reads them
--    changes. The SaaS Night cover tag (NC, coloured NLK) is not a CRE shift
--    and is untouched.
--
-- 2. CRE_REGULAR_AEST, "AEST regular hours", chip AEST: the Australia team's
--    ordinary weekday, as Regular hours (IST) is the Sri Lankan teams' and
--    Americas regular hours (PDT) the Americas team's. Standing hours, not a
--    turn. It is stored as 06:00-09:00 IST, the hours of the Morning
--    rotation, which is where their day overlaps the Sri Lankan one: the day
--    view lists them in the Morning card's list, tagged AEST, so the team can
--    see they are there. It is not a rotation (no reminder email, no
--    headcount) and not an escalation window, so Case Alerts never pages from
--    it. It has its own colour (token AEST), apart from IST and PDT.
--
-- 3. The Australia team's ordinary day becomes AEST regular hours. Matched by
--    name or key, as 0203 matches the Americas team (keys differ between
--    environments). Where no such team is in the Team Schedule, nothing is
--    added. A default already set to anything but the Morning window is left
--    as a rota admin chose it.
--
-- Safe to re-run: each tag changes only where it still has the old value, the
-- window is added once, and the default moves only from the Morning window.

UPDATE team_schedule_shift AS s
   SET short_code = v.new_code,
       updated_on = now(),
       updated_by = 'migration:0269_team_schedule_zone_tags'
  FROM (VALUES
        ('CRE_REGULAR',          'LK',        'IST'),
        ('CRE_REGULAR_IND',      'IND',       'IST'),
        ('CRE_AMERICAS',         'NLK',       'PDT'),
        ('CRE_AMERICAS_PT',      'NLK',       'PDT'),
        ('CRE_WEEKEND_NIGHT',    'NLK-WE',    'PDT-WE'),
        ('CRE_WEEKEND_NIGHT_OC', 'NLK-WE-OC', 'PDT-WE-OC')
       ) AS v(code, old_code, new_code)
 WHERE s.code = v.code
   AND s.short_code = v.old_code;

INSERT INTO team_schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, is_rotation, required_headcount,
     short_code, colour_token, sort_order, created_by, updated_by)
VALUES
    ('CRE_REGULAR_AEST', 'AEST regular hours', 'CRE', NULL, NULL, 'WEEKDAY', 360, 540,
     FALSE, FALSE, FALSE, NULL,
     'AEST', 'AEST', 36, 'migration:0269_team_schedule_zone_tags', 'migration:0269_team_schedule_zone_tags')
ON CONFLICT (code) DO NOTHING;

-- A database that took an earlier draft of this file coloured it like the India
-- region shift; AEST has its own colour.
UPDATE team_schedule_shift
   SET colour_token = 'AEST', updated_on = now(),
       updated_by = 'migration:0269_team_schedule_zone_tags'
 WHERE code = 'CRE_REGULAR_AEST' AND colour_token = 'IND';

INSERT INTO team_schedule_team_default_shift (team_key, shift_code, created_by)
SELECT t.key, 'CRE_REGULAR_AEST', 'migration:0269_team_schedule_zone_tags'
  FROM team t
 WHERE t.key IS NOT NULL
   AND (lower(t.key) ~ '^australia\M' OR lower(t.name) ~ '^australia\M')
ON CONFLICT (team_key) DO UPDATE
   SET shift_code = EXCLUDED.shift_code
 WHERE team_schedule_team_default_shift.shift_code = 'CRE_MORNING';
