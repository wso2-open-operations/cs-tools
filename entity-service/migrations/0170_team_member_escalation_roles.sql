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

-- The call-escalation ladder reaches five rungs, and 'member'/'lead' can only
-- tell two of them apart. An ABT has no sub teams: its three sub leads and the
-- one lead above them differ by rank inside one flat team, so the distinction
-- has to live on the membership row. The two heads sit in their own leadership
-- team rather than an ABT, which is what keeps a head resolving to no ABT for
-- /users/me -- the absence the Team Schedule page already reads as "belongs to
-- neither group".
--
-- 'member' becomes 'engineer' because everyone holding it is one, CRE or SRE.
--
-- This is the rank a rung escalates BY, and it is deliberately not the same
-- axis as the rota admin roles 0156 adds. Those are a granted permission --
-- may this person edit that rota -- and they live in role/user_role, where a
-- grant is auditable and revocable per environment. This is a position in one
-- team's ladder, which is a property of the membership itself and travels with
-- it. Someone can hold either without the other.
--
-- seed-team-schedule.sql asks pg_constraint which vocabulary is live rather
-- than naming a value, so the seed keeps working whichever of the two schemas
-- a database is on. That check reads the constraint this file rewrites, so
-- this migration must land before the seed runs -- which it does, the seed
-- being a separate step after every migration.
--
-- The constraint is dropped BEFORE the UPDATE: the UPDATE writes a value the
-- old CHECK forbids.
--
-- Re-runnable: the DROP is guarded, the UPDATE matches nothing on a second
-- pass, and the ADD follows a DROP of the same name.
ALTER TABLE team_member DROP CONSTRAINT IF EXISTS team_member_role_check;

UPDATE team_member SET role = 'engineer' WHERE role = 'member';
ALTER TABLE team_member ALTER COLUMN role SET DEFAULT 'engineer';

ALTER TABLE team_member ADD CONSTRAINT team_member_role_check
    CHECK (role IN ('engineer', 'sub_lead', 'lead', 'cre_head', 'cs_head'));
