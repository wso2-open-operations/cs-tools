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

-- MERGE / DEPLOY CHECK for migration 0282. Not a migration: never place this
-- under migrations/.
--
-- Why: 0282 was changed during review (PR #3321) so its backfill uses the
-- same team filter as the service (rosteredTeamWhere): a lead of a team that
-- works no rota, such as a change approval board, no longer gives their tags
-- a family. Migrations are tracked by filename, so a database that already
-- ran the earlier version of 0282 will not run the corrected one.
--
-- How: run each step in order (psql or DBeaver). Steps 1 and 2 only read.
--
--   psql -X -v ON_ERROR_STOP=1 -f scripts/check_0282_custom_tag_family_backfill.sql

-- Step 1. Has 0282 run here?
--   No row  -> nothing to check: `make migrate` runs the corrected file. Stop.
--   One row -> it ran (maybe the earlier version). Go on to step 2.
SELECT filename, applied_at
  FROM csm_migration_applied_migration
 WHERE filename = '0282_team_schedule_custom_tags_per_family.sql';

-- Step 2. Tags the earlier version gave a family that the corrected one
-- would not: custom tags created before 0282 ran, whose creator is neither a
-- rota admin of that family nor the lead of a rota team of that family.
--   0 rows  -> the earlier version gave the same result. Done.
--   Rows    -> review each; a family set on purpose since then looks the same.
--              Then use step 3 for the ones to put back to shared.
SELECT k.code, k.label, k.family, k.created_by, k.created_on
  FROM team_schedule_absence_kind k
  JOIN csm_migration_applied_migration m
    ON m.filename = '0282_team_schedule_custom_tags_per_family.sql'
 WHERE k.family IS NOT NULL
   AND k.created_on < m.applied_at
   AND k.created_by IS DISTINCT FROM 'migration'
   AND k.moves_to_team_key IS NULL
   AND NOT EXISTS (                       -- not a rota admin of that family
        SELECT 1
          FROM "user" u
          JOIN user_role ur ON ur.user_id = u.id
          JOIN role ro ON ro.id = ur.role_id
         WHERE lower(u.email) = lower(k.created_by)
           AND ro.name = lower(k.family::text) || '_rota_admin')
   AND NOT EXISTS (                       -- not a lead of a rota team of that family
        SELECT 1
          FROM "user" u
          JOIN team_member tm ON tm.user_id = u.id AND tm.role IN ('lead', 'americas_team_lead')
          JOIN team t ON t.id = tm.team_id
         WHERE lower(u.email) = lower(k.created_by)
           AND CASE WHEN lower(t.type) LIKE 'sre%' THEN 'SRE'
                    WHEN lower(t.type) LIKE 'sme%' THEN 'SME'
                    ELSE 'CRE' END = k.family::text
           -- Same team test as rosteredTeamWhere in schedule_repo.go.
           AND t.type IS NOT NULL AND lower(t.type) LIKE ANY (ARRAY['cre%', 'sre%', 'sme%'])
           AND lower(t.type) NOT LIKE '%leadership%' AND lower(COALESCE(t.name, '')) NOT LIKE '%leadership%'
           AND (lower(t.type) LIKE '%-abt'
                OR EXISTS (SELECT 1 FROM team_schedule_rota rr WHERE lower(rr.team_type) = lower(t.type) AND rr.is_active)
                OR EXISTS (SELECT 1 FROM team_schedule_team_default_shift dd WHERE dd.team_key = t.key)
                OR EXISTS (SELECT 1 FROM team_schedule_absence_kind kk WHERE lower(kk.moves_to_team_key) = lower(t.key))
                OR EXISTS (SELECT 1 FROM team_schedule_assignment aa WHERE aa.team_key = t.key)
                OR EXISTS (SELECT 1 FROM team_schedule_absence bb WHERE bb.team_key = t.key)))
 ORDER BY k.family, k.code;

-- Step 3. ONLY if step 2 returned rows you want shared again. Kept commented
-- out so running this file never writes. Put the codes from step 2 in the
-- list, run it, check the SELECT, then change ROLLBACK to COMMIT and run again.
--
-- BEGIN;
-- UPDATE team_schedule_absence_kind
--    SET family = NULL, updated_on = NOW(), updated_by = '0282-review-fix'
--  WHERE code IN ('CODE_1', 'CODE_2')
--    AND family IS NOT NULL;
-- SELECT code, family, updated_by FROM team_schedule_absence_kind WHERE code IN ('CODE_1', 'CODE_2');
-- ROLLBACK;
