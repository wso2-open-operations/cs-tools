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

-- Team Schedule: somebody moved to another team for a span stays a member of
-- their own team.
--
-- A move tag (Brazil rotation onto Americas, Migration, SaaS Night cover, the
-- India team) files a span under the team it moves them to and names their own
-- team in home_team_key; their membership never changes (0202). Some people
-- were loaded as members of the team they were moved to instead, and not of
-- their own. The roster then had nobody to change their own team's days: the
-- row was read-only for that team's lead and its blank weekdays were not drawn.
--
-- For each such membership this puts them back on their own team and takes
-- them off the move's team. Only where the data says their own team is still
-- theirs, so a real transfer to that team after an old move is left alone:
--   * they are a member of a team a move tag moves people to (T), and not of
--     the home team their latest span onto T names (H);
--   * a span onto T naming H runs today or later, or something is filed under
--     H after their last span onto T ended (leave, a turn);
--   * the membership on T is an engineer or sub_lead (a lead decides who Case
--     Alerts calls and is never moved);
--   * T and H are CRE teams (a team whose type is neither SRE nor SME, as the
--     schedule decides a team's family). SRE's SaaS Night cover and every SME
--     team are left as they are.
-- The membership on T stays when it carries an alert tier, a rota manager
-- grant, or a turn on T from today on that a lead placed by hand (a MOVE turn
-- is the span's own and goes with it); they are then on both teams, which a
-- rota admin can finish from Rota Access.
--
-- Data only: no table, column, type or constraint changes. Names nobody.
-- Re-runnable: a second run finds them on their own team and changes nothing.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TEMP TABLE _moved_member ON COMMIT DROP AS
WITH target AS (
    SELECT DISTINCT lower(k.moves_to_team_key) AS key
      FROM team_schedule_absence_kind k
     WHERE k.moves_to_team_key IS NOT NULL
),
span AS (
    SELECT ab.user_id, lower(ab.team_key) AS target_key, lower(ab.home_team_key) AS home_key,
           ab.starts_on, ab.ends_on,
           row_number() OVER (PARTITION BY ab.user_id, lower(ab.team_key)
                              ORDER BY ab.starts_on DESC, ab.ends_on DESC NULLS FIRST) AS latest
      FROM team_schedule_absence ab
      JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
     WHERE lower(k.moves_to_team_key) = lower(ab.team_key)
       AND ab.home_team_key IS NOT NULL
       AND lower(ab.home_team_key) <> lower(ab.team_key)
)
SELECT m.id AS member_id, m.user_id, m.role, m.alert_tier,
       tt.id AS target_id, tt.key AS target_key, ht.id AS home_id, ht.key AS home_key
  FROM team_member m
  JOIN team tt ON tt.id = m.team_id
  JOIN target ON target.key = lower(tt.key)
  JOIN span s ON s.user_id = m.user_id AND s.target_key = lower(tt.key) AND s.latest = 1
  JOIN team ht ON lower(ht.key) = s.home_key
 WHERE m.role IN ('engineer', 'sub_lead')
   AND lower(COALESCE(tt.type, '')) NOT LIKE 'sre%' AND lower(COALESCE(tt.type, '')) NOT LIKE 'sme%'
   AND lower(COALESCE(ht.type, '')) NOT LIKE 'sre%' AND lower(COALESCE(ht.type, '')) NOT LIKE 'sme%'
   AND NOT EXISTS (SELECT 1 FROM team_member h WHERE h.team_id = ht.id AND h.user_id = m.user_id)
   AND (
        EXISTS (SELECT 1 FROM span a
                 WHERE a.user_id = m.user_id AND a.target_key = lower(tt.key) AND a.home_key = s.home_key
                   AND (a.ends_on IS NULL OR a.ends_on >= CURRENT_DATE))
     OR EXISTS (SELECT 1 FROM team_schedule_absence o
                 WHERE o.user_id = m.user_id AND lower(o.team_key) = s.home_key
                   AND o.starts_on > (SELECT MAX(a.ends_on) FROM span a
                                       WHERE a.user_id = m.user_id AND a.target_key = lower(tt.key)))
     OR EXISTS (SELECT 1 FROM team_schedule_assignment o
                 WHERE o.user_id = m.user_id AND lower(o.team_key) = s.home_key
                   AND o.rota_date > (SELECT MAX(a.ends_on) FROM span a
                                       WHERE a.user_id = m.user_id AND a.target_key = lower(tt.key)))
   );

-- Back on their own team, as the same engineer or sub_lead.
INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role, group_id)
SELECT gen_random_uuid(), now(), now(), 'migration:0274', 'migration:0274', mm.home_id, mm.user_id, mm.role,
       (SELECT g.id FROM "group" g WHERE g.id = mm.home_id)
  FROM _moved_member mm;

-- Off the move's team, unless something there still needs the membership.
DELETE FROM team_member m
 USING _moved_member mm
 WHERE m.id = mm.member_id
   AND mm.alert_tier IS NULL
   AND NOT EXISTS (SELECT 1 FROM team_schedule_rota_marker r
                    WHERE r.user_id = mm.user_id AND lower(r.team_key) = lower(mm.target_key))
   AND NOT EXISTS (SELECT 1 FROM team_schedule_assignment a
                    WHERE a.user_id = mm.user_id AND lower(a.team_key) = lower(mm.target_key)
                      AND a.source IS DISTINCT FROM 'MOVE'
                      AND a.rota_date >= CURRENT_DATE);

COMMIT;
