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

-- Rotas: the named rotations inside a family, so SRE can run SaaS (Apollo &
-- Artemis) and IaaS side by side, and SME can run one rotation per product.
--
-- Until now a family had exactly one set of zones -- SRE's TZ1-TZ3 -- and the
-- day view drew a lane for every zone of the family. A second SRE rotation
-- seeded as more zones would have landed in SaaS's ladder. A rota groups its
-- own zones, carries the rules the source doc states for it (how often the
-- duty changes hands, the escalation interval), and names the team.type its
-- teams have, the same convention family already follows (CRE-ABT / SRE-ABT).
-- A team's rota is therefore read, never stored on team: teams arrive from the
-- ServiceNow sync, and a mapping table someone has to keep in step is exactly
-- how a team would end up on no rota at all.
--
-- Additive only. Nothing existing is renamed, removed or re-typed:
--   * team_schedule_rota is new;
--   * team_schedule_zone.rota_id is new and NULLABLE (TZ1-TZ3 are backfilled
--     to SRE_SAAS, but a zone without a rota still works as before);
--   * the zone-family CHECK is widened from SRE to SRE or SME -- every
--     existing row already satisfies the wider rule;
--   * the new zones and windows are new rows with new codes.
-- Source of the rotations: "CSM SRE + SME on call". Times are LK time, which
-- is the shifts' authoring time zone (Asia/Colombo), in minutes from midnight;
-- an end past 1440 runs into the next day, as SRE_TZ3 already does.
--
-- One transaction with a short lock timeout, as the other schedule
-- migrations; safe to re-run.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS team_schedule_rota (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         VARCHAR(255) NULL,
    updated_by         VARCHAR(255) NULL,
    code               VARCHAR(16)  NOT NULL UNIQUE,
    label              VARCHAR(100) NOT NULL,
    family             team_schedule_shift_family_enum NOT NULL,
    -- The team.type the rota's teams carry (compared case-insensitively). One
    -- type belongs to one rota, so a team is never on two.
    team_type          VARCHAR(50)  NOT NULL,
    -- How often the L1/L2 duty changes hands, as the rota's own sheet says.
    rotates            VARCHAR(10)  NOT NULL DEFAULT 'WEEKLY',
    -- Minutes before an unanswered page moves up a level; NULL where the
    -- source does not say. Informational: the escalation ladder's timing is
    -- still its own (csm-notification-service policy.go).
    escalation_minutes SMALLINT     NULL,
    source_sheet       VARCHAR(200) NULL,
    sort_order         SMALLINT     NOT NULL DEFAULT 0,
    is_active          BOOLEAN      NOT NULL DEFAULT TRUE,
    CONSTRAINT team_schedule_rota_rotates_check
        CHECK (rotates IN ('DAILY', 'WEEKLY', 'IRREGULAR')),
    CONSTRAINT team_schedule_rota_escalation_minutes_check
        CHECK (escalation_minutes IS NULL OR escalation_minutes > 0)
);
CREATE UNIQUE INDEX IF NOT EXISTS team_schedule_rota_team_type_unique
    ON team_schedule_rota (lower(team_type));

ALTER TABLE team_schedule_zone
    ADD COLUMN IF NOT EXISTS rota_id UUID NULL REFERENCES team_schedule_rota (id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_team_schedule_zone_rota_id ON team_schedule_zone (rota_id);

-- Zones were SRE's alone; SME rotations work in zones too (a Day and a Night
-- one each). Dropped by definition rather than by name, because the
-- constraint carries an older name on a database that came up through the
-- pre-prefix chain.
DO $$
DECLARE c record;
BEGIN
    FOR c IN SELECT conname FROM pg_constraint
              WHERE conrelid = 'team_schedule_shift'::regclass AND contype = 'c'
                AND pg_get_constraintdef(oid) LIKE '%zone_id IS NULL%'
                AND pg_get_constraintdef(oid) LIKE '%family%'
    LOOP
        EXECUTE format('ALTER TABLE team_schedule_shift DROP CONSTRAINT %I', c.conname);
    END LOOP;
END $$;
ALTER TABLE team_schedule_shift
    ADD CONSTRAINT team_schedule_shift_zone_family_check
    CHECK (zone_id IS NULL OR family IN ('SRE', 'SME'));

-- ── The rotas ─────────────────────────────────────────────────────────────
-- SRE_SAAS is the rota that exists today; IaaS and the SME rotations are new.
-- PaaS SRE is N/A in the source doc, so it has no rota until it has a schedule.
INSERT INTO team_schedule_rota
    (code, label, family, team_type, rotates, escalation_minutes, source_sheet, sort_order, created_by, updated_by)
VALUES
    ('SRE_SAAS',       'SaaS · Apollo & Artemis',     'SRE', 'sre-abt',            'IRREGULAR',  5, '[SaaS] [Automation] ABT-Rotation',                       10, 'migration', 'migration'),
    ('SRE_IAAS',       'IaaS',                        'SRE', 'sre-iaas',           'DAILY',      5, 'IAAS Incident Handling Rotation',                        20, 'migration', 'migration'),
    ('SME_ASGARDEO',   'Asgardeo',                    'SME', 'sme-asgardeo',       'DAILY',      5, '[Asgardeo] Incident Handling Rotation',                  10, 'migration', 'migration'),
    ('SME_CHOREO',     'Choreo Runtime',              'SME', 'sme-choreo-runtime', 'WEEKLY',  NULL, 'Incident Handling Rotation - APIM & Integration Cloud',  20, 'migration', 'migration'),
    ('SME_BIJIRA',     'Bijira',                      'SME', 'sme-bijira',         'WEEKLY',  NULL, 'Incident Handling Rotation - APIM & Integration Cloud',  30, 'migration', 'migration'),
    ('SME_DEVANT',     'Devant',                      'SME', 'sme-devant',         'WEEKLY',  NULL, 'Incident Handling Rotation - APIM & Integration Cloud',  40, 'migration', 'migration'),
    ('SME_CLOUD_AGENT','WSO2 Cloud · Agent platform', 'SME', 'sme-cloud-agent',    'WEEKLY',     5, 'WSO2 Cloud Rotation',                                    50, 'migration', 'migration'),
    ('SME_CLOUD_CORE', 'WSO2 Cloud · Core',           'SME', 'sme-cloud-core',     'WEEKLY',     5, 'WSO2 Cloud Rotation',                                    60, 'migration', 'migration'),
    ('SME_MOESIF',     'Moesif',                      'SME', 'sme-moesif',         'WEEKLY',    30, 'Moesif Special Ops Rotation',                            70, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

-- SaaS SRE's zones belong to the SaaS rota. Only a zone with no rota yet.
UPDATE team_schedule_zone z SET rota_id = r.id, updated_on = NOW()
  FROM team_schedule_rota r
 WHERE r.code = 'SRE_SAAS' AND z.code IN ('TZ1', 'TZ2', 'TZ3') AND z.rota_id IS NULL;

-- ── Their zones: a Day and a Night one per rotation ───────────────────────
-- A rotation runs the same shape every day of the week, so a zone is its own
-- weekend zone (as TZ3 is), and nothing folds at the weekend.
INSERT INTO team_schedule_zone (code, label, sort_order, rota_id, created_by, updated_by)
SELECT v.code, v.label, v.sort_order, r.id, 'migration', 'migration'
FROM (VALUES
    ('IAAS_D', 'IaaS · Day',          21, 'SRE_IAAS'),
    ('IAAS_N', 'IaaS · Night',        22, 'SRE_IAAS'),
    ('ASG_D',  'Asgardeo · Day',      31, 'SME_ASGARDEO'),
    ('ASG_N',  'Asgardeo · Night',    32, 'SME_ASGARDEO'),
    ('CRT_D',  'Choreo Runtime · Day',   41, 'SME_CHOREO'),
    ('CRT_N',  'Choreo Runtime · Night', 42, 'SME_CHOREO'),
    ('BIJ_D',  'Bijira · Day',        51, 'SME_BIJIRA'),
    ('BIJ_N',  'Bijira · Night',      52, 'SME_BIJIRA'),
    ('DVT_D',  'Devant · Day',        61, 'SME_DEVANT'),
    ('DVT_N',  'Devant · Night',      62, 'SME_DEVANT'),
    ('WCA_D',  'Agent platform · Day',   71, 'SME_CLOUD_AGENT'),
    ('WCA_N',  'Agent platform · Night', 72, 'SME_CLOUD_AGENT'),
    ('WCC_D',  'Cloud Core · Day',    81, 'SME_CLOUD_CORE'),
    ('WCC_N',  'Cloud Core · Night',  82, 'SME_CLOUD_CORE'),
    ('MOE_D',  'Moesif · Day',        91, 'SME_MOESIF'),
    ('MOE_N',  'Moesif · Night',      92, 'SME_MOESIF')
) AS v(code, label, sort_order, rota_code)
JOIN team_schedule_rota r ON r.code = v.rota_code
ON CONFLICT (code) DO NOTHING;

UPDATE team_schedule_zone SET weekend_zone_id = id, updated_on = NOW()
 WHERE code IN ('IAAS_D','IAAS_N','ASG_D','ASG_N','CRT_D','CRT_N','BIJ_D','BIJ_N',
                'DVT_D','DVT_N','WCA_D','WCA_N','WCC_D','WCC_N','MOE_D','MOE_N')
   AND weekend_zone_id IS NULL;

-- ── Their windows ─────────────────────────────────────────────────────────
-- One escalation window per zone, every day (ANY), tier left to the
-- assignment -- L1, L2 or L3 -- as SRE_TZ3 does. is_rotation, because being on
-- the rotation is the point; the no-overlap rule then keeps one person off a
-- Day and a Night that touch.
--
-- IaaS's two windows are seeded INACTIVE. The portal before this change reads
-- every zoned SRE window as a lane of SaaS's day, week and roster, and offers
-- it in SaaS's picker; the catalogue serves only active windows, so while that
-- portal may still be live -- this migration can run before the new webapp
-- ships -- IaaS must not be one. Switch them on once the rota-aware webapp is
-- deployed, with the IaaS teams' team.type (docs/team-schedule.md, Rotas):
--   UPDATE team_schedule_shift SET is_active = TRUE, updated_on = NOW()
--    WHERE code IN ('SRE_IAAS_DAY', 'SRE_IAAS_NIGHT');
-- SME's windows need no such wait: that portal knows only CRE and SRE, and
-- passes SME windows by.
INSERT INTO team_schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, is_rotation, required_headcount,
     short_code, colour_token, sort_order, is_active, created_by, updated_by)
SELECT v.code, v.label, v.family::team_schedule_shift_family_enum, z.id,
       NULL, 'ANY'::team_schedule_day_scope_enum, v.start_minute, v.end_minute,
       FALSE, TRUE, TRUE, NULL,
       v.short_code, v.colour_token, v.sort_order,
       v.code NOT LIKE 'SRE\_IAAS\_%',   -- IaaS waits for the new portal; see above
       'migration', 'migration'
FROM (VALUES
    ('SRE_IAAS_DAY',   'IaaS day escalation',              'SRE', 'IAAS_D',  480, 1200, 'Day',   'TZ1', 210),
    ('SRE_IAAS_NIGHT', 'IaaS night escalation',            'SRE', 'IAAS_N', 1200, 1920, 'Night', 'TZ3', 220),
    ('SME_ASG_DAY',    'Asgardeo day escalation',          'SME', 'ASG_D',   570, 1110, 'Day',   'TZ1', 310),
    ('SME_ASG_NIGHT',  'Asgardeo night escalation',        'SME', 'ASG_N',  1110, 2010, 'Night', 'TZ3', 320),
    ('SME_CRT_DAY',    'Choreo Runtime day escalation',    'SME', 'CRT_D',   360, 1080, 'Day',   'TZ1', 410),
    ('SME_CRT_NIGHT',  'Choreo Runtime night escalation',  'SME', 'CRT_N',  1080, 1800, 'Night', 'TZ3', 420),
    ('SME_BIJ_DAY',    'Bijira day escalation',            'SME', 'BIJ_D',   360, 1080, 'Day',   'TZ1', 510),
    ('SME_BIJ_NIGHT',  'Bijira night escalation',          'SME', 'BIJ_N',  1080, 1800, 'Night', 'TZ3', 520),
    ('SME_DVT_DAY',    'Devant day escalation',            'SME', 'DVT_D',   360, 1080, 'Day',   'TZ1', 610),
    ('SME_DVT_NIGHT',  'Devant night escalation',          'SME', 'DVT_N',  1080, 1800, 'Night', 'TZ3', 620),
    ('SME_WCA_DAY',    'Agent platform day escalation',    'SME', 'WCA_D',   360, 1080, 'Day',   'TZ1', 710),
    ('SME_WCA_NIGHT',  'Agent platform night escalation',  'SME', 'WCA_N',  1080, 1800, 'Night', 'TZ3', 720),
    ('SME_WCC_DAY',    'Cloud Core day escalation',        'SME', 'WCC_D',   360, 1080, 'Day',   'TZ1', 810),
    ('SME_WCC_NIGHT',  'Cloud Core night escalation',      'SME', 'WCC_N',  1080, 1800, 'Night', 'TZ3', 820),
    ('SME_MOE_DAY',    'Moesif day escalation',            'SME', 'MOE_D',   600, 1320, 'Day',   'TZ1', 910),
    ('SME_MOE_NIGHT',  'Moesif night escalation',          'SME', 'MOE_N',  1320, 2040, 'Night', 'TZ3', 920)
) AS v(code, label, family, zone_code, start_minute, end_minute, short_code, colour_token, sort_order)
JOIN team_schedule_zone z ON z.code = v.zone_code
ON CONFLICT (code) DO NOTHING;

-- ── Who may edit any SME rota ─────────────────────────────────────────────
-- The same capability cre_rota_admin and sre_rota_admin give their families
-- (migration 0156); granting it to people stays a deployment step.
INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
VALUES (gen_random_uuid(), now(), now(), 'migration', 'migration',
        'sme_rota_admin', 'May edit the rota of any SME team, not only their own')
ON CONFLICT (name) DO NOTHING;

COMMIT;
