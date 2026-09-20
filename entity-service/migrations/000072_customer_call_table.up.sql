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
    CREATE TYPE customer_call_state_enum AS ENUM (
        'CUSTOMER_REJECTED',
        'PENDING_ON_CUSTOMER',
        'SCHEDULED',
        'CONCLUDED',
        'NOTES_PENDING',
        'PENDING_ON_WSO2',
        'WSO2_REJECTED',
        'CANCELED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- assignment_group: SKIP FOR NOW, matching sc_catalog/cmdb_ci_service precedent
-- of deferring reference fields not yet needed downstream.
CREATE TABLE IF NOT EXISTS customer_call (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(100),
    assigned_to_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    account_contact_id UUID REFERENCES account_contact(id) ON DELETE SET NULL,
    work_item_id UUID REFERENCES work_item(id) ON DELETE CASCADE,
    call_accepted_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    opened_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    opened_on TIMESTAMPTZ,
    closed_on TIMESTAMPTZ,
    due_on TIMESTAMPTZ,
    scheduled_on TIMESTAMPTZ,
    is_active BOOLEAN,
    state customer_call_state_enum,
    duration INTERVAL,
    action_items TEXT,
    all_notes TEXT,
    call_access_details TEXT,
    plan TEXT,
    attendees TEXT,
    calendar_event_id VARCHAR(100),
    reason TEXT,
    requester VARCHAR(100),
    actual_call_duration VARCHAR(100),
    final_times JSONB,
    call_link TEXT,
    meeting_due_date VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_customer_call_assigned_to_id ON customer_call (assigned_to_id);
CREATE INDEX IF NOT EXISTS idx_customer_call_account_contact_id ON customer_call (account_contact_id);
CREATE INDEX IF NOT EXISTS idx_customer_call_work_item_id ON customer_call (work_item_id);
CREATE INDEX IF NOT EXISTS idx_customer_call_call_accepted_by_id ON customer_call (call_accepted_by_id);
CREATE INDEX IF NOT EXISTS idx_customer_call_opened_by_id ON customer_call (opened_by_id);
CREATE INDEX IF NOT EXISTS idx_customer_call_closed_by_id ON customer_call (closed_by_id);
