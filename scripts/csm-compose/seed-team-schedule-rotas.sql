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

-- Local stack only: one team per new rota (migrations 0199-0200), so the SRE
-- and SME rota pickers have something to show. Team rows only -- no people
-- and no generated rota: seed-team-schedule.sql generates CRE- and SRE-shaped
-- rotas by family, and an SME rotation is neither. Roster people onto these
-- from the month roster, or import a real rota.
--
-- A team's rota is read from its type (team_schedule_rota.team_type), so the
-- types below are what puts each team on its rota. Safe to re-run.
--   psql "$DATABASE_URL" -f scripts/csm-compose/seed-team-schedule-rotas.sql
BEGIN;

INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, key, type)
SELECT md5('seed-team-'||v.key)::uuid, now(), now(), 'seed', 'seed', v.display, v.key, v.type
FROM (VALUES
    ('iaas',           'IaaS',                        'sre-iaas'),
    ('asgardeo',       'Asgardeo',                    'sme-asgardeo'),
    ('choreo-runtime', 'Choreo Runtime',              'sme-choreo-runtime'),
    ('bijira',         'Bijira',                      'sme-bijira'),
    ('devant',         'Devant',                      'sme-devant'),
    ('cloud-agent',    'WSO2 Cloud · Agent platform', 'sme-cloud-agent'),
    ('cloud-core',     'WSO2 Cloud · Core',           'sme-cloud-core'),
    ('moesif',         'Moesif',                      'sme-moesif')
) AS v(key, display, type)
ON CONFLICT (id) DO NOTHING;

-- 0200 seeds IaaS's windows inactive for the deploy window (see its header);
-- a local stack runs the rota-aware webapp, so they are switched on here.
UPDATE team_schedule_shift SET is_active = TRUE, updated_on = NOW()
 WHERE code IN ('SRE_IAAS_DAY', 'SRE_IAAS_NIGHT') AND NOT is_active;

COMMIT;
