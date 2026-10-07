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

-- Team Schedule: the standing hours a team works on an ordinary weekday, and
-- Americas cover as weekday hours.
--
-- The roster draws a weekday nobody has marked as a working day. That was
-- Regular hours (LK) for every team, which is wrong for the Americas team: it
-- always covers the Americas' hours, so each member's ordinary day is
-- Americas cover (NLK). A team with no row here keeps Regular hours.
--
-- A table of the schedule's own rather than a column on team, which is shared
-- with the directory sync and is not the schedule's to widen.
CREATE TABLE IF NOT EXISTS team_schedule_team_default_shift (
    team_key    VARCHAR(64) PRIMARY KEY REFERENCES team(key) ON UPDATE CASCADE ON DELETE CASCADE,
    shift_code  VARCHAR(32) NOT NULL REFERENCES team_schedule_shift(code) ON UPDATE CASCADE,
    created_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  VARCHAR(255)
);

-- The Americas team, matched by name the way 0202 matches it: team keys
-- differ between environments ("americas", "americas_cre_team"). Where no team
-- matches, or the catalogue has no Americas cover window, nothing is added.
INSERT INTO team_schedule_team_default_shift (team_key, shift_code, created_by)
SELECT t.key, 'CRE_AMERICAS', 'migration'
  FROM team t
 WHERE t.key IS NOT NULL
   AND (lower(t.name) ~ '^americas([ _-]+cre)?([ _-]+team)?$' OR lower(t.key) ~ '^americas([ _-]+cre)?([ _-]+team)?$')
   AND EXISTS (SELECT 1 FROM team_schedule_shift s WHERE s.code = 'CRE_AMERICAS')
ON CONFLICT (team_key) DO NOTHING;

-- Americas cover is the Americas team's ordinary weekday, the counterpart of
-- Regular hours on the Sri Lankan teams -- and like Regular hours it is not
-- worked at the weekend, which has its own Americas weekend rotation. It was
-- seeded for any day, so marking somebody on it over a range put it on every
-- Saturday and Sunday too. Day scope is not copied onto an assignment, so it
-- may change on a window already in use; existing rows are left as they are.
UPDATE team_schedule_shift
   SET day_scope = 'WEEKDAY'
 WHERE code = 'CRE_AMERICAS'
   AND day_scope <> 'WEEKDAY';
