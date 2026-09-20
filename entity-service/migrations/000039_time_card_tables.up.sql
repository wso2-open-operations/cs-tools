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
    CREATE TYPE time_card_state_enum AS ENUM (
        'PENDING', 'SUBMITTED', 'APPROVED', 'REJECTED', 'RECALLED', 'PROCESSED', 'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE time_card_issue_complexity_enum AS ENUM (
        'NOT_APPLICABLE', 'LOW', 'MEDIUM', 'HIGH'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- category is not synced at all: this table only ever receives
-- category=task_work rows (see time_card.yaml's source_filter), so a
-- category column would be redundant/constant.
-- customer_project_id references "project", not a "customer_project" table - the latter
-- is only this repo's job_key/source_table name for that data (see customer_project.yaml).
CREATE TABLE IF NOT EXISTS time_card (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    case_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    customer_project_id UUID REFERENCES project(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    approved_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    work_date DATE,
    is_billable BOOLEAN,
    state time_card_state_enum,
    issue_complexity time_card_issue_complexity_enum,
    analyzing_minutes INTEGER,
    setting_up_minutes INTEGER,
    reproducing_debugging_minutes INTEGER,
    providing_solution_minutes INTEGER,
    patching_minutes INTEGER,
    work_log_comment TEXT,
    lead_comment TEXT
);

CREATE INDEX IF NOT EXISTS idx_time_card_case_id ON time_card (case_id);
CREATE INDEX IF NOT EXISTS idx_time_card_customer_project_id ON time_card (customer_project_id);
CREATE INDEX IF NOT EXISTS idx_time_card_user_id ON time_card (user_id);
CREATE INDEX IF NOT EXISTS idx_time_card_approved_by_id ON time_card (approved_by_id);

-- approver_list fans out via expand_list (one time_card row -> N rows here). id is a
-- single deterministic UUID (not a composite key) because PgLoader.Upsert always does
-- ON CONFLICT (id) - see upsertSQL/upsertRowSQL in internal/loader. created_on/updated_on/
-- created_by/updated_by are copied from the parent time_card row per expanded approver,
-- matching project_group_role's precedent (one SN audit trail per bundle record, not per
-- pairing).
CREATE TABLE IF NOT EXISTS time_card_approver (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    time_card_id UUID NOT NULL REFERENCES time_card(id) ON DELETE CASCADE,
    approver_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    UNIQUE (time_card_id, approver_id)
);

CREATE INDEX IF NOT EXISTS idx_time_card_approver_time_card_id ON time_card_approver (time_card_id);
CREATE INDEX IF NOT EXISTS idx_time_card_approver_approver_id ON time_card_approver (approver_id);
