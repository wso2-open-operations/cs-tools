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

-- work_item type extension for PROBLEM_TASK, same shared-primary-key pattern
-- as incident_task/change_task: id IS work_item.id, ON DELETE CASCADE, no
-- audit columns - those live on work_item and are reachable via join.
-- No endpoint reads or writes this yet -- same "schema only, unimplemented"
-- posture as change_task/communication_plan (see this repo's own CLAUDE.md,
-- "Incident, Problem, IncidentTask, and Conversation"): added here for
-- parity with operations/csm-sync-service's own copy of this schema, not
-- because a consuming feature exists today.
DO $$ BEGIN
    CREATE TYPE problem_task_state_enum AS ENUM (
        'PENDING', 'OPEN', 'WORK_IN_PROGRESS', 'CLOSED_COMPLETE', 'CLOSED_INCOMPLETE', 'CLOSED_SKIPPED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE problem_task_type_enum AS ENUM ('GENERAL', 'MODEL', 'RCA');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS problem_task (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    opened_on TIMESTAMPTZ,
    opened_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    state problem_task_state_enum,
    problem_id UUID REFERENCES problem(id) ON DELETE SET NULL,
    problem_task_type problem_task_type_enum
);

CREATE INDEX IF NOT EXISTS idx_problem_task_opened_by_id ON problem_task (opened_by_id);
CREATE INDEX IF NOT EXISTS idx_problem_task_problem_id ON problem_task (problem_id);
