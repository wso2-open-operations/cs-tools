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

-- work_item type extension for INCIDENT_ALERT_TASK, same shared-primary-key
-- pattern as incident/problem/change_task: id IS work_item.id, ON DELETE
-- CASCADE, no audit columns - those live on work_item and are reachable via
-- join.
DO $$ BEGIN
    CREATE TYPE incident_alert_task_state_enum AS ENUM (
        'PENDING', 'OPEN', 'IN_PROGRESS', 'COMPLETE', 'SKIPPED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS incident_alert_task (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    opened_on TIMESTAMPTZ,
    due_on TIMESTAMPTZ,
    due_duration INTERVAL,
    state incident_alert_task_state_enum,
    "order" INTEGER,
    communication_plan_id UUID REFERENCES communication_plan(id) ON DELETE SET NULL,
    communication_task_definition_id UUID REFERENCES communication_task_definition(id) ON DELETE SET NULL,
    incident_alert_id UUID REFERENCES incident_alert(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_incident_alert_task_communication_plan_id ON incident_alert_task (communication_plan_id);
CREATE INDEX IF NOT EXISTS idx_incident_alert_task_communication_task_definition_id ON incident_alert_task (communication_task_definition_id);
CREATE INDEX IF NOT EXISTS idx_incident_alert_task_incident_alert_id ON incident_alert_task (incident_alert_id);
