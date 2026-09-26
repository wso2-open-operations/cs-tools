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

-- Closes four gaps found by checking the schema against every code the Team
-- Schedule UI actually renders:
--
--   1. Allocation kinds were an ENUM. The UI has eleven of them and the list
--      grows -- a new one should be a row a lead can add, not an ALTER TYPE
--      and a deploy. schedule_shift was already a table for exactly this
--      reason; this makes the two consistent.
--   2. A standing allocation ("on Migration until further notice") had no way
--      to be open-ended, because ends_on was NOT NULL.
--   3. The chip each roster cell draws carries a SHORT code and a colour,
--      both distinct from code and label, and neither had anywhere to live.
--   4. Three windows the UI uses were missing from the catalogue, and the
--      weekend rotation was seeded with the wrong hours.

-- ── 1 + 3. allocation and leave kinds become a table ──────────────────────

CREATE TABLE IF NOT EXISTS schedule_absence_kind (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(255),
    updated_by    VARCHAR(255),
    code          VARCHAR(32) NOT NULL UNIQUE,
    short_code    VARCHAR(12) NOT NULL,
    label         VARCHAR(100) NOT NULL,
    -- 'LEAVE' is time off and cannot be claimed against a weekend; 'ALLOCATION'
    -- is work done elsewhere; 'EXCLUDED' is off the rota entirely.
    bucket        VARCHAR(16) NOT NULL,
    colour_token  VARCHAR(24) NOT NULL,
    sort_order    SMALLINT NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT schedule_absence_kind_bucket_check
        CHECK (bucket IN ('LEAVE', 'ALLOCATION', 'EXCLUDED'))
);

INSERT INTO schedule_absence_kind (code, short_code, label, bucket, colour_token, sort_order, created_by, updated_by)
VALUES
    ('ANNUAL_LEAVE', 'AL',   'Annual leave',           'LEAVE',      'AL',  10, 'migration', 'migration'),
    ('LIEU_LEAVE',   'LL',   'Lieu leave',             'LEAVE',      'LL',  20, 'migration', 'migration'),
    ('RND',          'R&D',  'R&D',                    'ALLOCATION', 'RND', 30, 'migration', 'migration'),
    ('CUSTOMER',     'CUS',  'Customer allocation',    'ALLOCATION', 'EXT', 40, 'migration', 'migration'),
    ('ALLO_EXT',     'EXT',  'Allocated — external',   'ALLOCATION', 'EXT', 50, 'migration', 'migration'),
    ('ALLO_INT',     'INT',  'Allocated — internal',   'ALLOCATION', 'INT', 60, 'migration', 'migration'),
    ('ALLO_BR',      'BR',   'Allocated — BR',         'ALLOCATION', 'BR',  70, 'migration', 'migration'),
    ('ALLO_CRIS',    'CRIS', 'Allocated — CRIS',       'ALLOCATION', 'BR',  80, 'migration', 'migration'),
    ('MIGRATION',    'Mig',  'Migration',              'ALLOCATION', 'MIG', 90, 'migration', 'migration'),
    ('ONBOARDING',   'ONB',  'Onboarding',             'ALLOCATION', 'ONB',100, 'migration', 'migration'),
    ('EXCLUDED',     'EXC',  'Excluded from rota',     'EXCLUDED',   'EXC',110, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

-- Swap the enum column for a reference, carrying any existing rows across by
-- code. The two enum labels that were renamed map explicitly.
ALTER TABLE schedule_absence ADD COLUMN IF NOT EXISTS kind_id UUID REFERENCES schedule_absence_kind(id) ON DELETE RESTRICT;

UPDATE schedule_absence a SET kind_id = k.id
FROM schedule_absence_kind k
WHERE a.kind_id IS NULL
  AND k.code = CASE a.kind::text
                 WHEN 'RND_ALLOCATION'      THEN 'RND'
                 WHEN 'CUSTOMER_ALLOCATION' THEN 'CUSTOMER'
                 ELSE a.kind::text
               END;

ALTER TABLE schedule_absence ALTER COLUMN kind_id SET NOT NULL;
ALTER TABLE schedule_absence DROP COLUMN IF EXISTS kind;
DROP TYPE IF EXISTS schedule_absence_kind_enum;

CREATE INDEX IF NOT EXISTS idx_schedule_absence_kind_id ON schedule_absence (kind_id);

-- ── 2. a standing allocation runs until further notice ────────────────────
-- NULL means open-ended. daterange already reads NULL as unbounded, so the
-- GiST index and every overlap query keep working untouched.
ALTER TABLE schedule_absence ALTER COLUMN ends_on DROP NOT NULL;
ALTER TABLE schedule_absence DROP CONSTRAINT IF EXISTS schedule_absence_range_check;
ALTER TABLE schedule_absence ADD CONSTRAINT schedule_absence_range_check
    CHECK (ends_on IS NULL OR ends_on >= starts_on);

-- ── 3. the chip a roster cell draws ───────────────────────────────────────
ALTER TABLE schedule_shift ADD COLUMN IF NOT EXISTS short_code   VARCHAR(12);
ALTER TABLE schedule_shift ADD COLUMN IF NOT EXISTS colour_token VARCHAR(24);

UPDATE schedule_shift s SET short_code = v.short_code, colour_token = v.colour_token, updated_on = NOW()
FROM (VALUES
    ('CRE_MORNING',    '6-9a', 'AM'),
    ('CRE_MORNING_OC', '6-9a', 'AM'),
    ('CRE_REGULAR',    'LK',   'LK'),
    ('CRE_EVENING',    '6-9p', 'PM'),
    ('CRE_AMERICAS',   'NLK',  'NLK'),
    ('CRE_WEEKEND',    'WE',   'WE'),
    ('SRE_REGULAR',    'REG',  'LK'),
    ('SRE_TZ1_L1',     'L1',   'L1'),
    ('SRE_TZ1',        'TZ1',  'TZ1'),
    ('SRE_TZ2_L1',     'L1',   'L1'),
    ('SRE_TZ2',        'TZ2',  'TZ2'),
    ('SRE_TZ3',        'TZ3',  'TZ3'),
    ('SRE_WE_TZ1',     'TZ1',  'TZ1'),
    ('SRE_WE_TZ2',     'TZ2',  'TZ2')
) AS v(code, short_code, colour_token)
WHERE s.code = v.code;

-- ── 4. the windows the UI uses that the catalogue was missing ─────────────
-- The weekend rotation is drawn 06:00-21:00, not 09:00-18:00: the earlier
-- seed guessed, and the UI's own band says otherwise.
UPDATE schedule_shift
   SET start_minute = 360, end_minute = 1260, updated_on = NOW()
 WHERE code = 'CRE_WEEKEND';

INSERT INTO schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute,
     is_on_call, is_escalation, short_code, colour_token, sort_order, created_by, updated_by)
VALUES
    ('CRE_WEEKEND_NIGHT',    'Weekend non-LK cover', 'CRE', NULL, NULL, 'WEEKEND', 1260, 1800, FALSE, FALSE, 'NLK-WE',    'NLKWE', 70, 'migration', 'migration'),
    ('CRE_WEEKEND_NIGHT_OC', 'Weekend on-call',      'CRE', NULL, NULL, 'WEEKEND', 1260, 1800, TRUE,  FALSE, 'NLK-WE-OC', 'OC',    80, 'migration', 'migration'),
    ('CRE_REGULAR_IND',      'India region shift',   'CRE', NULL, NULL, 'WEEKDAY',  540, 1080, FALSE, FALSE, 'IND',       'IND',   35, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE schedule_shift ALTER COLUMN short_code SET NOT NULL;
ALTER TABLE schedule_shift ALTER COLUMN colour_token SET NOT NULL;
