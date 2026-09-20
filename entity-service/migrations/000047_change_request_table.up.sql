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
    CREATE TYPE change_request_impact_enum AS ENUM ('LOW', 'MEDIUM', 'HIGH');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_state_enum AS ENUM (
        'NEW', 'ASSESS', 'AUTHORIZE', 'CUSTOMER_APPROVAL', 'SCHEDULED', 'IMPLEMENT',
        'REVIEW', 'CUSTOMER_REVIEW', 'ROLLBACK', 'CLOSED', 'CANCELED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_priority_enum AS ENUM ('LOW', 'CRITICAL', 'MODERATE', 'HIGH');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_category_enum AS ENUM (
        'SOFTWARE', 'NETWORK', 'SERVICE', 'TELECOM', 'HARDWARE', 'SYSTEM_SOFTWARE',
        'DOCUMENTATION', 'APPLICATIONS_SOFTWARE', 'OTHER'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_risk_enum AS ENUM ('HIGH', 'MODERATE', 'LOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_approval_enum AS ENUM ('REQUESTED', 'APPROVED', 'REJECTED', 'NOT_REQUESTED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_type_enum AS ENUM ('INFRA', 'GENERAL');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_likelihood_enum AS ENUM ('LOW', 'MEDIUM', 'HIGH');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE change_request_confirmation_enum AS ENUM ('DISAGREE', 'AGREE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Shared-primary-key extension of work_item, same pattern as "case" (see
-- 000018_case_table.up.sql): change_request.id IS work_item.id. The FK's ON DELETE
-- CASCADE is why change_request_details.yaml doesn't need its own delete_sync -
-- deleting the work_item row (via change_request.yaml's delete_sync) cascades here.
-- change_request_type isn't named "type" because work_item already has its own
-- (unrelated) type column and the two would be easy to confuse when joined.
CREATE TABLE IF NOT EXISTS change_request (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    start_on TIMESTAMPTZ,
    end_on TIMESTAMPTZ,
    calendar_duration INTERVAL,
    impact change_request_impact_enum,
    state change_request_state_enum,
    priority change_request_priority_enum,
    category change_request_category_enum,
    risk change_request_risk_enum,
    closed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    requested_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    approval change_request_approval_enum,
    justification TEXT,
    impact_description TEXT,
    service_outage_downtime TEXT,
    communication_plan TEXT,
    rollback_process TEXT,
    test_plan TEXT,
    implementation_plan TEXT,
    affected_services TEXT,
    affected_component TEXT,
    rollback_duration VARCHAR(255),
    is_customer_approved BOOLEAN,
    is_customer_reviewed BOOLEAN,
    change_request_type change_request_type_enum,
    likelihood change_request_likelihood_enum,
    is_planning_visible_to_customers BOOLEAN,
    customer_updated_date_confirmation change_request_confirmation_enum,
    customer_updated_on TIMESTAMPTZ,
    work_start_on TIMESTAMPTZ,
    work_end_on TIMESTAMPTZ,
    git_reference TEXT,
    risk_impact_analysis VARCHAR(4000),
    backout_plan VARCHAR(4000)
);

CREATE INDEX IF NOT EXISTS idx_change_request_closed_by_user_id ON change_request (closed_by_user_id);
CREATE INDEX IF NOT EXISTS idx_change_request_requested_by_user_id ON change_request (requested_by_user_id);
