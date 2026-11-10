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

-- Skill management revamp (M3): removing a skill from a profile now sets
-- active = FALSE instead of deleting the row, so one (person, skill) pair keeps
-- one own row for life, and the database itself refuses a second active one.
-- Run scripts/skill_data_checks.sh (B3) first.

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_skill_own_active
    ON user_skill (user_id, skill_id) WHERE active AND NOT inherited;

-- The directory search starts from a skill and finds its holders.
CREATE INDEX IF NOT EXISTS idx_user_skill_skill_user_active
    ON user_skill (skill_id, user_id) WHERE active;
