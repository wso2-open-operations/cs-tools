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

-- Reuses case_escalation_level_enum from 000018_case_table.up.sql: this table
-- shares the identical value set for current_level/previous_level.
DO $$ BEGIN
    CREATE TYPE case_escalation_level_enum AS ENUM (
        'EL0', 'EL1', 'EL2', 'EL3', 'EL4', 'EL5'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS case_escalation (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    current_level case_escalation_level_enum,
    previous_level case_escalation_level_enum,
    reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_case_escalation_work_item_id ON case_escalation (work_item_id);

-- Junction table fanned out from u_notification_list via expand_list, same shape
-- as work_item_watcher (see 000040_work_item_watcher_table.up.sql): one row per
-- (escalation, notified user) pair.
CREATE TABLE IF NOT EXISTS case_escalation_notification_list (
    id UUID PRIMARY KEY,
    case_escalation_id UUID NOT NULL REFERENCES case_escalation(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    UNIQUE (case_escalation_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_case_escalation_notification_list_case_escalation_id ON case_escalation_notification_list (case_escalation_id);
CREATE INDEX IF NOT EXISTS idx_case_escalation_notification_list_user_id ON case_escalation_notification_list (user_id);
