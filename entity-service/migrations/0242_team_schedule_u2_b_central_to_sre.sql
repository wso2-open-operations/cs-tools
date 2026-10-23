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

-- Team Schedule: U2 and B-Central move from the SME family to the SRE family.
--
-- 0217 added them as SME rotations, but no escalation reaches them: they are
-- the teams behind the "U2 SRE Group" and "B-Central SRE Group" incidents, so
-- they are paged on the SRE ladder the moment such an incident is created,
-- each from its own rota, as IaaS is. The SRE ladder only reads SRE windows,
-- and an SRE group's alerts can only be sent to an SRE team, so the family is
-- what has to change.
--
-- Data only: no table, column, type or constraint is created or changed.
--
--   * team.type sme-u2 -> sre-u2 and sme-b-central -> sre-b-central: a team's
--     family is its type's prefix;
--   * their rotas (SME_U2, SME_B_CENTRAL) become SRE rotas on the new types.
--     The codes stay: they are keys, scripts outside this repository name
--     them, and the chain the alert service and the portal show is named from
--     them (U2-SRE, B-Central-SRE), not from the code's prefix;
--   * their four windows become SRE windows. Hours, zones, tiers and codes
--     stay. A used window's family is frozen by the used-shift guard (0153)
--     because an assignment must never say something its window no longer
--     does; no assignment holds a family -- it is read from the window -- so
--     there is nothing to recompute, and this says
--     team_schedule.allow_shift_rewrite for the family alone.
--
-- Every turn already rostered on these rotas stays, with its tier. The SRE
-- ladder calls the person holding L1, L2, L3; someone rostered with no tier
-- is not called (the SME page counted them as L1), so each turn needs a tier.
--
-- Re-runnable: each row is changed only while it still holds the SME value,
-- so a second run changes nothing. One transaction with a short lock timeout,
-- as the other schedule migrations.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL team_schedule.allow_shift_rewrite = 'on';

-- ── The teams ─────────────────────────────────────────────────────────────
UPDATE team t
   SET type = v.new_type, updated_on = NOW(), updated_by = 'migration'
  FROM (VALUES
    ('u2',        'sme-u2',        'sre-u2'),
    ('b-central', 'sme-b-central', 'sre-b-central')
) AS v(key, old_type, new_type)
 WHERE lower(t.key) = v.key
   AND lower(t.type) = v.old_type;

-- ── The rotas ─────────────────────────────────────────────────────────────
UPDATE team_schedule_rota r
   SET family = 'SRE', team_type = v.new_type, updated_on = NOW(), updated_by = 'migration'
  FROM (VALUES
    ('SME_U2',        'sre-u2'),
    ('SME_B_CENTRAL', 'sre-b-central')
) AS v(code, new_type)
 WHERE r.code = v.code
   AND (r.family IS DISTINCT FROM 'SRE' OR r.team_type IS DISTINCT FROM v.new_type);

-- ── The windows ───────────────────────────────────────────────────────────
UPDATE team_schedule_shift s
   SET family = 'SRE', updated_on = NOW(), updated_by = 'migration'
 WHERE s.code IN ('SME_U2_DAY', 'SME_U2_NIGHT', 'SME_BCN_DAY', 'SME_BCN_NIGHT')
   AND s.family IS DISTINCT FROM 'SRE';

COMMIT;
