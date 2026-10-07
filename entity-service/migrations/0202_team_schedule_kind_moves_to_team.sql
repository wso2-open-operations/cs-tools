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

-- Team Schedule: a tag that moves somebody to another team for its span.
--
-- The Brazil rotation (BR) and Migration are long stints -- months, not days
-- -- spent working for another team: someone on BR works the Americas team's
-- rota, someone on Migration works for the Migration team. Marked as an
-- allocation against their own ABT, they sat in that ABT's rows as "away" for
-- the whole stint, could not be rostered on the team they were actually
-- working for, and the escalation ladder skipped them even on that team's
-- shifts.
--
-- A tag now names the team it moves people to. A span of such a tag is filed
-- under that team (team_key), with the team the person came from beside it
-- (home_team_key), so:
--   * the roster shows them under the team they are working for, for exactly
--     those dates, and back under their own team either side of the span --
--     nothing to "move back": their membership never changes;
--   * that team's lead may roster them while the span lasts;
--   * the home team's lead still owns the span itself, and may end or move it;
--   * the ladder treats them as away from every team but that one.
-- works_rota_there says whether the stint is rota work on that team (BR: the
-- Americas rota) or work beside any rota (Migration). Only rota work keeps
-- them out of the day view's off-rota list.
--
-- shows_as_shift_code is the standing window the stint's days are drawn as on
-- the roster: on that team the person simply works its normal hours --
-- Americas cover (NLK) on the Brazil rotation, Regular hours (LK) on
-- Migration -- and a column of "BR" said nothing a reader needed. The span is
-- still what the cell opens, so leave can be marked over it and it can end.
--
-- Which team each tag moves people to is data, not code. The two built-in
-- tags are matched to their teams by name, because team keys differ between
-- environments (one database spells the Americas team "americas", another
-- "americas_cre_team"); where no team matches, the tag is left as it was and
-- an admin can point it at the right team later.
ALTER TABLE team_schedule_absence_kind
    ADD COLUMN IF NOT EXISTS moves_to_team_key VARCHAR(64)
        REFERENCES team(key) ON UPDATE CASCADE ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS works_rota_there BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS shows_as_shift_code VARCHAR(32)
        REFERENCES team_schedule_shift(code) ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE team_schedule_absence
    ADD COLUMN IF NOT EXISTS home_team_key VARCHAR(64)
        REFERENCES team(key) ON UPDATE CASCADE ON DELETE SET NULL;

-- "Americas", "americas", "Americas_cre_team" and "Americas CRE team" all
-- name the one team; "Americas Leadership" would not. The first match in key
-- order wins, so a database with two candidates still gets one answer.
UPDATE team_schedule_absence_kind k
   SET moves_to_team_key = t.key,
       works_rota_there = v.works_rota_there,
       updated_on = NOW(),
       updated_by = 'migration'
  FROM (VALUES
    ('ALLO_BR',   '^americas([ _-]+cre)?([ _-]+team)?$',  TRUE),
    ('MIGRATION', '^migration([ _-]+cre)?([ _-]+team)?$', FALSE)
  ) AS v(code, name_pattern, works_rota_there)
  CROSS JOIN LATERAL (
    SELECT tm.key FROM team tm
     WHERE tm.key IS NOT NULL
       AND (lower(tm.name) ~ v.name_pattern OR lower(tm.key) ~ v.name_pattern)
     ORDER BY tm.key
     LIMIT 1
  ) t
 WHERE k.code = v.code
   AND k.moves_to_team_key IS NULL;

-- The window each stint's days are drawn as, where the catalogue has it.
UPDATE team_schedule_absence_kind k
   SET shows_as_shift_code = v.shift_code,
       updated_on = NOW(),
       updated_by = 'migration'
  FROM (VALUES ('ALLO_BR', 'CRE_AMERICAS'), ('MIGRATION', 'CRE_REGULAR')) AS v(code, shift_code)
 WHERE k.code = v.code
   AND k.shows_as_shift_code IS NULL
   AND EXISTS (SELECT 1 FROM team_schedule_shift s WHERE s.code = v.shift_code);

-- Spans already recorded with those tags were filed under the person's own
-- team. Move each to the team its tag now names, keeping the team it was filed
-- under as its home -- the same shape a new span takes. Only a span not yet
-- under its tag's team is touched, so a re-run changes nothing.
UPDATE team_schedule_absence ab
   SET home_team_key = ab.team_key,
       team_key = k.moves_to_team_key,
       updated_on = NOW(),
       updated_by = 'migration'
  FROM team_schedule_absence_kind k
 WHERE k.id = ab.kind_id
   AND k.moves_to_team_key IS NOT NULL
   AND lower(ab.team_key) <> lower(k.moves_to_team_key);
