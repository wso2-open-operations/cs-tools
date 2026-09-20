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

DO $$ BEGIN
    CREATE TYPE sla_policy_target_enum AS ENUM ('WORKAROUND', 'RESPONSE', 'RESOLUTION');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE sla_policy_reset_action_enum AS ENUM ('CANCEL_EXISTING_TASK_SLA', 'COMPLETE_EXISTING_TASK_SLA');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE sla_policy_when_to_cancel_enum AS ENUM (
        'START_CONDITIONS_ARE_NOT_MET', 'CANCEL_CONDITIONS_ARE_MET', 'NEVER'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE sla_policy_when_to_resume_enum AS ENUM ('PAUSE_CONDITIONS_ARE_NOT_MET', 'RESUME_CONDITIONS_ARE_MET');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Enum constants are SNAKE_UPPER_CASE of ServiceNow's choice LABEL text (apostrophes
-- dropped), not the underlying dot-walked VALUE - e.g. label "The caller's time zone"
-- (value task.caller_id.time_zone) becomes THE_CALLERS_TIME_ZONE.
DO $$ BEGIN
    CREATE TYPE sla_policy_timezone_source_enum AS ENUM (
        'THE_CALLERS_LOCATION_TIME_ZONE', 'THE_CALLERS_TIME_ZONE', 'THE_SLA_DEFINITIONS_TIME_ZONE',
        'THE_CIS_LOCATION_TIME_ZONE', 'THE_TASKS_LOCATION_TIME_ZONE'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- schedule is dot-walked to schedule.name at extraction time (see
-- sla_policy.yaml) and stored as a plain display string, not an FK - CLAUDE.md
-- §8 forbids requesting sysparm_display_value for the whole payload, but
-- dot-walking a single reference field's display column avoids that entirely.
CREATE TABLE IF NOT EXISTS sla_policy (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN,
    target sla_policy_target_enum,
    duration INTERVAL,
    schedule VARCHAR(255),
    retroactive BOOLEAN,
    retroactive_pause BOOLEAN,
    reset_action sla_policy_reset_action_enum,
    when_to_cancel sla_policy_when_to_cancel_enum,
    when_to_resume sla_policy_when_to_resume_enum,
    start_condition TEXT,
    stop_condition TEXT,
    timezone_source sla_policy_timezone_source_enum,
    cancel_condition TEXT,
    resume_condition TEXT,
    pause_condition TEXT
);
