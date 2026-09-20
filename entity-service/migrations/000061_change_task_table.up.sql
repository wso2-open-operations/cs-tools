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

-- work_item type extension for CHANGE_TASK, same shared-primary-key pattern
-- as incident/problem (000058, 000059): id IS work_item.id, ON DELETE
-- CASCADE, no audit columns - those live on work_item and are reachable via
-- join. assigned_to_id/parent_id/business_duration already live on work_item,
-- not this table.
DO $$ BEGIN
    CREATE TYPE change_task_type_enum AS ENUM ('PLANNING', 'IMPLEMENTATION', 'TESTING', 'REVIEW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_task_created_from_enum AS ENUM ('WORKFLOW', 'MANUAL', 'FLOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_task_state_enum AS ENUM ('PENDING', 'OPEN', 'IN_PROGRESS', 'CLOSED', 'CANCELED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS change_task (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    closed_on TIMESTAMPTZ,
    due_on TIMESTAMPTZ,
    opened_on TIMESTAMPTZ,
    expected_start_on TIMESTAMPTZ,
    change_request_id UUID REFERENCES change_request(id) ON DELETE SET NULL,
    opened_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    change_task_type change_task_type_enum,
    work_end_on TIMESTAMPTZ,
    created_from change_task_created_from_enum,
    close_notes TEXT,
    state change_task_state_enum,
    is_active BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_change_task_change_request_id ON change_task (change_request_id);
CREATE INDEX IF NOT EXISTS idx_change_task_opened_by_id ON change_task (opened_by_id);
CREATE INDEX IF NOT EXISTS idx_change_task_closed_by_id ON change_task (closed_by_id);
