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

-- Skill management revamp (M2): skill names are unique ignoring case and outer
-- spaces (until now only the service checked, under an advisory lock), and one
-- level scale is marked as the default: the scale a skill created from an
-- approved request gets, and the scale the request form offers.
-- Run scripts/skill_data_checks.sh (B1, R1) first.

CREATE UNIQUE INDEX IF NOT EXISTS ux_skill_name_lower ON skill (lower(btrim(name)));

ALTER TABLE skill_level_type ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX IF NOT EXISTS ux_skill_level_type_default ON skill_level_type (is_default) WHERE is_default;

-- The default is the scale most skills use (the oldest such on a tie), or the
-- oldest scale when no skill has one. Nothing changes when a default is set.
UPDATE skill_level_type
   SET is_default = TRUE, updated_on = NOW(), updated_by = 'skill-migration'
 WHERE NOT EXISTS (SELECT 1 FROM skill_level_type WHERE is_default)
   AND id = COALESCE(
         (SELECT s.level_type_id FROM skill s JOIN skill_level_type t ON t.id = s.level_type_id
           GROUP BY s.level_type_id, t.created_on ORDER BY count(*) DESC, t.created_on, s.level_type_id LIMIT 1),
         (SELECT id FROM skill_level_type ORDER BY created_on, id LIMIT 1));
