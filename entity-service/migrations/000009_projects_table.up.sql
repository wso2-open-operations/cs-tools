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
    CREATE TYPE wso2_closure_state_enum AS ENUM ('OPEN', 'READ_ONLY', 'CLOSED', 'RESTRICTED', 'SUSPENDED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE onboarding_status_enum AS ENUM ('NOT_STARTED', 'IN_PROGRESS', 'COMPLETED', 'ON_HOLD', 'NOT_APPLICABLE', 'EXPIRED', 'CANCELLED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE choreo_application_status_enum AS ENUM ('PENDING', 'CREATED_APPLICATION', 'SUBSCRIBED_APPLICATION', 'GENERATED_CREDENTIALS', 'COMPLETED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE invoice_due_date_closure_state_enum AS ENUM (
        'OPEN', 'NOTIFIED', 'NOTICED', 'RESTRICTED', 'SUSPENDED',
        'PENDING_NOTIFIED', 'PENDING_SUSPENDED', 'PENDING_NOTICED', 'PENDING_RESTRICTED',
        'NOTIFIED_AND_PREVIOUSLY_PAID', 'NOTICED_AND_PREVIOUSLY_PAID',
        'RESTRICTED_AND_PREVIOUSLY_PAID', 'SUSPENDED_AND_PREVIOUSLY_PAID',
        'PENDING_NOTIFIED_AND_PREVIOUSLY_PAID', 'PENDING_NOTICED_AND_PREVIOUSLY_PAID',
        'PENDING_RESTRICTED_AND_PREVIOUSLY_PAID', 'PENDING_SUSPENDED_AND_PREVIOUSLY_PAID'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE compliance_violation_closure_state_enum AS ENUM ('SUSPENDED', 'OPEN');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE end_date_closure_state_enum AS ENUM (
        'OPEN', 'NOTIFIED', 'CLOSURE_NOTICES', 'RESTRICTED', 'CLOSED',
        'PENDING_NOTIFIED', 'PENDING_CLOSURE_NOTICES', 'PENDING_CLOSED', 'PENDING_RESTRICTED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS project (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    key VARCHAR(100) NOT NULL UNIQUE,
    sf_id VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255),
    account_id UUID REFERENCES account(id),
    is_active BOOLEAN,
    planned_end_date TIMESTAMPTZ,
    start_date DATE,
    end_date DATE,
    consumed_duration INTERVAL,
    wso2_closure_state wso2_closure_state_enum,
    last_case_created_on TIMESTAMPTZ,
    choreo_application_id VARCHAR(255),
    consumption_tracking_file_generated_on TIMESTAMPTZ,
    remaining_onboarding_duration INTERVAL,
    total_onboarding_duration INTERVAL,
    client_id VARCHAR(255),
    client_secret VARCHAR(255),
    onboarding_status onboarding_status_enum,
    total_query_duration INTERVAL,
    is_pdp_subscription BOOLEAN,
    remaining_query_duration INTERVAL,
    description TEXT,
    choreo_application_status choreo_application_status_enum,
    onboarding_go_live_plan_date DATE,
    compliance_violation_date DATE,
    invoice_due_date_closure_state invoice_due_date_closure_state_enum,
    consumed_onboarding_duration INTERVAL,
    compliance_violation_closure_state compliance_violation_closure_state_enum,
    onboarding_expiry_date DATE,
    end_date_closure_state end_date_closure_state_enum,
    onboarding_go_live_date DATE
);

CREATE INDEX IF NOT EXISTS idx_project_account_id ON project (account_id);
