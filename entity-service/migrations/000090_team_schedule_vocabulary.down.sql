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

-- Rolling back narrows the vocabulary: any absence whose kind has no enum
-- equivalent (EXT, INT, BR, CRIS, Migration) cannot be represented, so those
-- rows are deleted rather than silently relabelled as something they are not.

DO $$ BEGIN
    CREATE TYPE schedule_absence_kind_enum AS ENUM (
        'ANNUAL_LEAVE', 'LIEU_LEAVE', 'RND_ALLOCATION', 'CUSTOMER_ALLOCATION',
        'ONBOARDING', 'EXCLUDED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DELETE FROM schedule_absence a USING schedule_absence_kind k
 WHERE a.kind_id = k.id
   AND k.code NOT IN ('ANNUAL_LEAVE','LIEU_LEAVE','RND','CUSTOMER','ONBOARDING','EXCLUDED');

ALTER TABLE schedule_absence ADD COLUMN IF NOT EXISTS kind schedule_absence_kind_enum;

UPDATE schedule_absence a SET kind = (CASE k.code
          WHEN 'RND'      THEN 'RND_ALLOCATION'
          WHEN 'CUSTOMER' THEN 'CUSTOMER_ALLOCATION'
          ELSE k.code END)::schedule_absence_kind_enum
FROM schedule_absence_kind k WHERE a.kind_id = k.id;

ALTER TABLE schedule_absence ALTER COLUMN kind SET NOT NULL;
DROP INDEX IF EXISTS idx_schedule_absence_kind_id;
ALTER TABLE schedule_absence DROP COLUMN IF EXISTS kind_id;
DROP TABLE IF EXISTS schedule_absence_kind;

-- an open-ended allocation cannot be expressed once ends_on is mandatory
DELETE FROM schedule_absence WHERE ends_on IS NULL;
ALTER TABLE schedule_absence DROP CONSTRAINT IF EXISTS schedule_absence_range_check;
ALTER TABLE schedule_absence ALTER COLUMN ends_on SET NOT NULL;
ALTER TABLE schedule_absence ADD CONSTRAINT schedule_absence_range_check
    CHECK (ends_on >= starts_on);

DELETE FROM schedule_shift WHERE code IN ('CRE_WEEKEND_NIGHT','CRE_WEEKEND_NIGHT_OC','CRE_REGULAR_IND');
UPDATE schedule_shift SET start_minute = 540, end_minute = 1080 WHERE code = 'CRE_WEEKEND';
ALTER TABLE schedule_shift DROP COLUMN IF EXISTS short_code;
ALTER TABLE schedule_shift DROP COLUMN IF EXISTS colour_token;
