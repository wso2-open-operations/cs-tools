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

-- work_item type extension for INCIDENT_TASK, same shared-primary-key
-- pattern as incident/incident_alert_task: id IS work_item.id, ON DELETE
-- CASCADE, no audit columns - those live on work_item and are reachable via
-- join.
DO $$ BEGIN
    CREATE TYPE incident_task_priority_enum AS ENUM ('CRITICAL', 'HIGH', 'MODERATE', 'LOW', 'PLANNING');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ServiceNow's choice list has "Open" for both 0 and 1, and one non-choice
-- garbage value ("SM") not modeled.
DO $$ BEGIN
    CREATE TYPE incident_task_state_enum AS ENUM (
        'PENDING', 'OPEN', 'WORK_IN_PROGRESS', 'CLOSED_COMPLETE', 'CLOSED_INCOMPLETE', 'CLOSED_SKIPPED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE incident_task_type_enum AS ENUM ('DEFAULT', 'INCIDENT_REPORT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS incident_task (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    opened_on TIMESTAMPTZ,
    closed_on TIMESTAMPTZ,
    close_notes TEXT,
    priority incident_task_priority_enum,
    state incident_task_state_enum,
    is_active BOOLEAN,
    opened_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    incident_id UUID REFERENCES incident(id) ON DELETE SET NULL,
    service_id UUID REFERENCES service(id) ON DELETE SET NULL,
    known_cause TEXT,
    next_steps TEXT,
    initial_action TEXT,
    affected_functionality TEXT,
    timeline TEXT,
    type incident_task_type_enum
);

CREATE INDEX IF NOT EXISTS idx_incident_task_opened_by_id ON incident_task (opened_by_id);
CREATE INDEX IF NOT EXISTS idx_incident_task_closed_by_id ON incident_task (closed_by_id);
CREATE INDEX IF NOT EXISTS idx_incident_task_incident_id ON incident_task (incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_task_service_id ON incident_task (service_id);
