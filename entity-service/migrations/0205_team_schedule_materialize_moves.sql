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
-- Team Schedule: real shifts for the moves already recorded.
--
-- From 0204 on, a span of a tag that moves someone to another team's rota
-- (works_rota_there, with a shows_as_shift_code: the Brazil rotation, worked as
-- Americas cover) writes that window as real shifts, source MOVE, on each day
-- of the span the window is worked. Spans recorded before then have none; this
-- creates them, so the day and week views, and the escalation ladder, see
-- those people on the Americas rota the way the roster already showed them.
--
-- A day that already holds an overlapping shift for the person is left as it
-- is (ON CONFLICT DO NOTHING covers the no-overlap exclusion constraint). An
-- open-ended span is filled to a year past the later of its start and today.
-- Removing such a span later clears through the person's last MOVE shift, so
-- whatever horizon it was written to is covered. Safe to re-run: a day
-- already written conflicts with its own row.
INSERT INTO team_schedule_assignment
    (id, created_on, updated_on, created_by, updated_by, user_id, team_id, team_key,
     shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, note)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration', 'migration', ab.user_id,
       (SELECT t.id FROM team t WHERE t.key = lower(ab.team_key)), ab.team_key,
       s.id, s.zone_id, s.tier, d::date,
       (d::date::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
       (d::date::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
       s.is_on_call, 'MOVE', NULL
  FROM team_schedule_absence ab
  JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
  JOIN team_schedule_shift s ON s.code = k.shows_as_shift_code
 CROSS JOIN LATERAL generate_series(
        ab.starts_on::timestamp,
        COALESCE(ab.ends_on, GREATEST(ab.starts_on, CURRENT_DATE) + 365)::timestamp,
        INTERVAL '1 day') AS d
 WHERE k.works_rota_there
   AND k.moves_to_team_key IS NOT NULL
   AND lower(ab.team_key) = lower(k.moves_to_team_key)
   AND (s.day_scope = 'ANY'
        OR (s.day_scope = 'WEEKDAY' AND extract(isodow FROM d) < 6)
        OR (s.day_scope = 'WEEKEND' AND extract(isodow FROM d) >= 6))
ON CONFLICT DO NOTHING;
