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

-- alert_duty: a standing nomination to take the first call for a team.
--
-- The escalation ladder's Level 0 is "first responders" -- the people already
-- watching when an incident lands. That is two different things united:
--
--   rota_members  whoever is rostered on the shift covering the incident.
--                 Time-dependent, and already answerable from
--                 team_schedule_assignment.
--   alert_duty    a nominated set of members of a particular team (an ABT, or
--                 Americas), T1/T2/T3. NOT time-dependent -- it is a standing
--                 property of the membership, which is why it belongs here
--                 and not on an assignment.
--
-- A separate column rather than more team_member.role values, for two
-- reasons. A nominee is still an engineer for every other purpose, and
-- role = 'alert_duty_t1' would lose that. And the two axes move
-- independently: adding a T4, or later letting a rank hold a nomination, is a
-- constraint change here rather than a data migration over role.
--
-- This is also not the rota's own tier (team_schedule_tier_enum, L1/L2/L3).
-- That one is the SRE ladder's, lives on an assignment, and is time-bound.
-- Same shape, different axis, deliberately different letters.
ALTER TABLE team_member ADD COLUMN IF NOT EXISTS alert_tier VARCHAR(2);

-- Postgres has no ADD CONSTRAINT IF NOT EXISTS; dropping first (a no-op the
-- first time) is what makes re-running this file safe.
ALTER TABLE team_member DROP CONSTRAINT IF EXISTS team_member_alert_tier_check;
ALTER TABLE team_member ADD CONSTRAINT team_member_alert_tier_check
    CHECK (alert_tier IS NULL OR alert_tier IN ('T1', 'T2', 'T3'));

-- A nominee is never a lead, stated by whoever owns the rules rather than
-- inferred. It matters structurally: a lead is who Level 1 and Level 2
-- escalate TO, so a lead also holding a Level 0 nomination would collapse two
-- rungs into one and quietly shorten the ladder -- the incident would reach
-- the same person twice and the escalation would look like it had climbed
-- when it had not.
ALTER TABLE team_member DROP CONSTRAINT IF EXISTS team_member_alert_tier_not_lead;
ALTER TABLE team_member ADD CONSTRAINT team_member_alert_tier_not_lead
    CHECK (alert_tier IS NULL OR role NOT IN ('lead', 'cre_head', 'cs_head'));

-- Exactly one person per tier per team: every ABT nominates three, T1, T2 and
-- T3, and so does the Americas team. That makes a rung's size predictable,
-- which is what the expected-call counts on each rule are checked against --
-- so it is worth the database refusing a second T1 rather than a rung quietly
-- growing and the first anybody hears of it being the bill.
CREATE UNIQUE INDEX IF NOT EXISTS team_member_alert_tier_unique
    ON team_member (team_id, alert_tier) WHERE alert_tier IS NOT NULL;
