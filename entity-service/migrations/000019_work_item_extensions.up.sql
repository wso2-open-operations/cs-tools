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

-- Four more work_item type extensions, same shared-primary-key pattern as
-- case (migrations/000018_case_table.up.sql): id IS work_item.id, ON DELETE
-- CASCADE, no audit columns - those live on work_item and are reachable
-- via join.

DO $$ BEGIN
    CREATE TYPE service_request_state_enum AS ENUM (
        'WORK_IN_PROGRESS', 'AWAITING_INFO', 'SOLUTION_PROPOSED', 'CLOSED', 'OPEN',
        'WAITING_ON_WSO2', 'REOPENED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE service_request_cause_enum AS ENUM (
        'SOLUTION_ARCHITECTURE', 'DEPLOYMENT_ARCHITECTURE', 'USER_ERROR_CONFIGURATION',
        'USER_ERROR_PRODUCT_CONCEPT', 'USER_ERROR_RUNTIME',
        'USER_ERROR_RECOMMENDATION_BEST_PRACTICES', 'CUSTOMIZATION_LIMITATION',
        'CUSTOMIZATION_BUG', 'DOCUMENTATION_GAP', 'DOCUMENTATION_ERROR', 'PRODUCT_LIMITATION',
        'PRODUCT_BUG', 'PRODUCT_REGRESSION', 'PRODUCT_MIGRATION', 'INFRASTRUCTURE_DATABASE',
        'INFRASTRUCTURE_NETWORK', 'INFRASTRUCTURE_JDK', 'INFRASTRUCTURE_LDAP',
        'INFRASTRUCTURE_LOAD_BALANCER', 'INFRASTRUCTURE_IAAS', 'INFRASTRUCTURE_EXTERNAL_PRODUCT',
        'INFRASTRUCTURE_PROXY', 'INFRASTRUCTURE_OS', 'INFRASTRUCTURE_OTHER', 'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS service_request (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    state service_request_state_enum,
    close_notes TEXT,
    cause service_request_cause_enum,
    closed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    resolved_on TIMESTAMPTZ,
    category VARCHAR(255),
    autoclosure_step VARCHAR(50),
    autoclosure_state_on TIMESTAMPTZ,
    json_data JSONB
);

CREATE INDEX IF NOT EXISTS idx_service_request_closed_by_user_id ON service_request (closed_by_user_id);

DO $$ BEGIN
    CREATE TYPE engagement_state_enum AS ENUM (
        'WORK_IN_PROGRESS', 'AWAITING_INFO', 'SOLUTION_PROPOSED', 'CLOSED', 'OPEN',
        'WAITING_ON_WSO2', 'REOPENED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE engagement_cause_enum AS ENUM (
        'SOLUTION_ARCHITECTURE', 'DEPLOYMENT_ARCHITECTURE', 'USER_ERROR_CONFIGURATION',
        'USER_ERROR_PRODUCT_CONCEPT', 'USER_ERROR_RUNTIME',
        'USER_ERROR_RECOMMENDATION_BEST_PRACTICES', 'CUSTOMIZATION_LIMITATION',
        'CUSTOMIZATION_BUG', 'DOCUMENTATION_GAP', 'DOCUMENTATION_ERROR', 'PRODUCT_LIMITATION',
        'PRODUCT_BUG', 'PRODUCT_REGRESSION', 'PRODUCT_MIGRATION', 'INFRASTRUCTURE_DATABASE',
        'INFRASTRUCTURE_NETWORK', 'INFRASTRUCTURE_JDK', 'INFRASTRUCTURE_LDAP',
        'INFRASTRUCTURE_LOAD_BALANCER', 'INFRASTRUCTURE_IAAS', 'INFRASTRUCTURE_EXTERNAL_PRODUCT',
        'INFRASTRUCTURE_PROXY', 'INFRASTRUCTURE_OS', 'INFRASTRUCTURE_OTHER', 'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE engagement_type_enum AS ENUM (
        'MIGRATION', 'CONSULTANCY', 'NEW_FEATURE_IMPROVEMENT', 'FOLLOW_UP', 'ONBOARDING'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE engagement_payment_type_enum AS ENUM ('PAID', 'FOC');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS engagement (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    state engagement_state_enum,
    close_notes TEXT,
    cause engagement_cause_enum,
    closed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    resolved_on TIMESTAMPTZ,
    autoclosure_step VARCHAR(50),
    autoclosure_state_on TIMESTAMPTZ,
    type engagement_type_enum,
    payment_type engagement_payment_type_enum,
    start_date DATE,
    end_date DATE
);

CREATE INDEX IF NOT EXISTS idx_engagement_closed_by_user_id ON engagement (closed_by_user_id);

DO $$ BEGIN
    CREATE TYPE security_report_analysis_state_enum AS ENUM (
        'WORK_IN_PROGRESS', 'AWAITING_INFO', 'SOLUTION_PROPOSED', 'CLOSED', 'OPEN',
        'WAITING_ON_WSO2', 'REOPENED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE security_report_analysis_cause_enum AS ENUM (
        'SOLUTION_ARCHITECTURE', 'DEPLOYMENT_ARCHITECTURE', 'USER_ERROR_CONFIGURATION',
        'USER_ERROR_PRODUCT_CONCEPT', 'USER_ERROR_RUNTIME',
        'USER_ERROR_RECOMMENDATION_BEST_PRACTICES', 'CUSTOMIZATION_LIMITATION',
        'CUSTOMIZATION_BUG', 'DOCUMENTATION_GAP', 'DOCUMENTATION_ERROR', 'PRODUCT_LIMITATION',
        'PRODUCT_BUG', 'PRODUCT_REGRESSION', 'PRODUCT_MIGRATION', 'INFRASTRUCTURE_DATABASE',
        'INFRASTRUCTURE_NETWORK', 'INFRASTRUCTURE_JDK', 'INFRASTRUCTURE_LDAP',
        'INFRASTRUCTURE_LOAD_BALANCER', 'INFRASTRUCTURE_IAAS', 'INFRASTRUCTURE_EXTERNAL_PRODUCT',
        'INFRASTRUCTURE_PROXY', 'INFRASTRUCTURE_OS', 'INFRASTRUCTURE_OTHER', 'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS security_report_analysis (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    state security_report_analysis_state_enum,
    close_notes TEXT,
    cause security_report_analysis_cause_enum,
    closed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    resolved_on TIMESTAMPTZ,
    autoclosure_step VARCHAR(50),
    autoclosure_state_on TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_security_report_analysis_closed_by_user_id ON security_report_analysis (closed_by_user_id);

DO $$ BEGIN
    CREATE TYPE announcement_state_enum AS ENUM ('OPEN', 'CLOSE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE announcement_cause_enum AS ENUM (
        'SOLUTION_ARCHITECTURE', 'DEPLOYMENT_ARCHITECTURE', 'USER_ERROR_CONFIGURATION',
        'USER_ERROR_PRODUCT_CONCEPT', 'USER_ERROR_RUNTIME',
        'USER_ERROR_RECOMMENDATION_BEST_PRACTICES', 'CUSTOMIZATION_LIMITATION',
        'CUSTOMIZATION_BUG', 'DOCUMENTATION_GAP', 'DOCUMENTATION_ERROR', 'PRODUCT_LIMITATION',
        'PRODUCT_BUG', 'PRODUCT_REGRESSION', 'PRODUCT_MIGRATION', 'INFRASTRUCTURE_DATABASE',
        'INFRASTRUCTURE_NETWORK', 'INFRASTRUCTURE_JDK', 'INFRASTRUCTURE_LDAP',
        'INFRASTRUCTURE_LOAD_BALANCER', 'INFRASTRUCTURE_IAAS', 'INFRASTRUCTURE_EXTERNAL_PRODUCT',
        'INFRASTRUCTURE_PROXY', 'INFRASTRUCTURE_OS', 'INFRASTRUCTURE_OTHER', 'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS announcement (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    state announcement_state_enum,
    close_notes TEXT,
    cause announcement_cause_enum,
    closed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    resolved_on TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_announcement_closed_by_user_id ON announcement (closed_by_user_id);
