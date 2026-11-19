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

-- Team Schedule: SaaS SRE weekend Day and Night windows, seeded INACTIVE.
--
-- SaaS SRE (Apollo and Artemis) moves its weekends from TZ1+2 (06:00-21:00,
-- SRE_WE_TZ1) and TZ3 (21:00-06:00, SRE_TZ3) to two shifts:
--
--   SRE_WE_DAY     Weekend Day     06:00-18:00 LK   zone TZ1, per team
--   SRE_WE_NIGHT   Weekend Night   18:00-06:00 LK   zone TZ3, one crew for the rota
--
-- Weekdays keep TZ1, TZ2 and TZ3. The new windows sit in the zones the
-- weekend already uses, so the roster's columns and lanes and the pager's
-- shared night crew (zone TZ3) need no change. The Night is
-- shared_across_teams, as SRE_TZ3 is (0223).
--
-- Both are inserted with is_active = FALSE. The catalogue serves only active
-- windows, so nothing offers, generates or pages them yet: the SaaS generator
-- keeps writing TZ1+2 / TZ3 weekends until both are served. Switching them on
-- is a separate migration, applied in the week before the first Day/Night
-- weekend (after Monday 06:00, before Saturday 06:00 IST), in one
-- transaction: SRE_WE_DAY and SRE_WE_NIGHT on, SRE_WE_TZ1 off, and
-- SRE_TZ3.day_scope = 'WEEKDAY'. None of those columns is one the used-shift
-- guard (0153) freezes. See entity-service/docs/team-schedule.md.
--
-- Data only; safe to re-run.
BEGIN;
SET LOCAL lock_timeout = '5s';

INSERT INTO team_schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, is_rotation, required_headcount,
     short_code, colour_token, sort_order, is_active, shared_across_teams,
     created_by, updated_by)
SELECT v.code, v.label, 'SRE'::team_schedule_shift_family_enum, z.id,
       NULL, 'WEEKEND'::team_schedule_day_scope_enum, v.start_minute, v.end_minute,
       FALSE, TRUE, TRUE, NULL,
       v.short_code, v.colour_token, v.sort_order, FALSE, v.shared_across_teams,
       'migration', 'migration'
FROM (VALUES
    ('SRE_WE_DAY',   'Weekend Day',   'TZ1',  360, 1080, 'Day',   'TZ1', 161, FALSE),
    ('SRE_WE_NIGHT', 'Weekend Night', 'TZ3', 1080, 1800, 'Night', 'TZ3', 162, TRUE)
) AS v(code, label, zone_code, start_minute, end_minute, short_code, colour_token, sort_order, shared_across_teams)
JOIN team_schedule_zone z ON z.code = v.zone_code
ON CONFLICT (code) DO NOTHING;

COMMIT;
