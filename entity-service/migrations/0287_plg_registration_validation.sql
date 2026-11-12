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

-- Registration validation: who set each playbook value, and the activity
-- summary the plg_registration_validation job derives them from.

DO $$ BEGIN
    CREATE TYPE plg_task_value_source_enum AS ENUM ('MANUAL', 'VALIDATION');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- NULL until someone writes the task. MANUAL is an engineer's write, cleared
-- values included; the job never overwrites it.
ALTER TABLE plg_playbook_run_task
    ADD COLUMN IF NOT EXISTS value_source plg_task_value_source_enum;
-- A local prototype added it as NOT NULL DEFAULT 'MANUAL'; align it.
ALTER TABLE plg_playbook_run_task
    ALTER COLUMN value_source DROP DEFAULT,
    ALTER COLUMN value_source DROP NOT NULL;

COMMENT ON COLUMN plg_playbook_run_task.value_source IS
    'Who last wrote the value: MANUAL (an engineer) or VALIDATION (the registration job). NULL when never written.';

-- Values written before this column existed were engineers'.
UPDATE plg_playbook_run_task
SET    value_source = 'MANUAL'
WHERE  value_source IS NULL
  AND  (value_bool IS NOT NULL OR value_number IS NOT NULL
        OR value_text IS NOT NULL OR value_checked IS NOT NULL);

-- One row per pairing, replaced on every run. Counts and steps only: no names,
-- emails or event payloads.
CREATE TABLE IF NOT EXISTS plg_activity_summary (
    org_platform_id UUID PRIMARY KEY REFERENCES plg_org_platform (id) ON DELETE CASCADE,
    window_start    TIMESTAMPTZ NOT NULL,
    window_end      TIMESTAMPTZ NOT NULL,
    first_event_on  TIMESTAMPTZ,
    last_event_on   TIMESTAMPTZ,
    event_count     INT         NOT NULL DEFAULT 0,
    -- {"created-agent": 3, ...}
    action_counts   JSONB       NOT NULL DEFAULT '{}'::JSONB,
    furthest_step   TEXT,
    computed_on     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_plg_activity_summary_window CHECK (window_end >= window_start),
    CONSTRAINT chk_plg_activity_summary_count  CHECK (event_count >= 0),
    CONSTRAINT chk_plg_activity_summary_counts CHECK (jsonb_typeof(action_counts) = 'object')
);
