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

-- Enum constants are SNAKE_UPPER_CASE of ServiceNow's choice LABEL text, not the
-- underlying value, same convention as sla_policy_target_enum (see
-- 000051_sla_policy_table.up.sql).
DO $$ BEGIN
    CREATE TYPE sla_stage_enum AS ENUM (
        'PAUSED', 'IN_PROGRESS', 'COMPLETED', 'ACHIEVED', 'BREACHED', 'CANCELLED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- schedule is dot-walked to schedule.name at extraction time (see sla.yaml) and
-- stored as a plain display string, not an FK - same convention as
-- sla_policy.schedule (see 000051_sla_policy_table.up.sql).
CREATE TABLE IF NOT EXISTS sla (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    work_item_id UUID REFERENCES work_item(id) ON DELETE CASCADE,
    sla_policy_id UUID REFERENCES sla_policy(id) ON DELETE SET NULL,
    schedule VARCHAR(255),
    is_active BOOLEAN,
    stage sla_stage_enum,
    has_breached BOOLEAN,
    timezone VARCHAR(100),
    start_on TIMESTAMPTZ,
    original_breached_on TIMESTAMPTZ,
    duration INTERVAL,
    business_duration INTERVAL,
    actual_elapsed_percentage NUMERIC(12,2),
    business_elapsed_percentage NUMERIC(12,2),
    end_on TIMESTAMPTZ,
    remaining_business_duration INTERVAL,
    planned_end_on TIMESTAMPTZ,
    remaining_actual_duration INTERVAL,
    pause_duration INTERVAL,
    business_pause_duration INTERVAL,
    paused_on TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sla_work_item_id ON sla (work_item_id);
CREATE INDEX IF NOT EXISTS idx_sla_sla_policy_id ON sla (sla_policy_id);
