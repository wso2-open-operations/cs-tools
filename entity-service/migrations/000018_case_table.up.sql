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
    CREATE TYPE case_severity_enum AS ENUM ('S0', 'S1', 'S2', 'S3', 'S4');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE case_issue_type_enum AS ENUM (
        'TOTAL_OUTAGE', 'PARTIAL_OUTAGE', 'PERFORMANCE_DEGRADATION', 'QUESTION',
        'SECURITY_OR_COMPLIANCE', 'ERROR'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE case_state_enum AS ENUM (
        'WORK_IN_PROGRESS', 'AWAITING_INFO', 'SOLUTION_PROPOSED', 'CLOSED', 'OPEN',
        'WAITING_ON_WSO2', 'REOPENED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE case_escalation_level_enum AS ENUM ('EL0', 'EL1', 'EL2', 'EL3', 'EL4', 'EL5');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE case_work_state_enum AS ENUM ('ONGOING', 'PAUSED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE case_resolution_code_enum AS ENUM (
        'SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED', 'SOLVED_WORKAROUND_PROVIDED',
        'SOLVED_BY_CUSTOMER', 'INCONCLUSIVE_OUT_OF_SCOPE', 'INCONCLUSIVE_CANNOT_REPRODUCE',
        'INCONCLUSIVE_NO_WORKAROUND', 'DUPLICATE_ISSUE', 'VOIDED_CANCELED',
        'SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT', 'SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET',
        'CONSIDERED_FOR_ROADMAP', 'ON_HOLD', 'SOLVED_FIXED_THE_ISSUE', 'SOLVED_BY_CONTRIBUTOR',
        'SOLVED_BY_NOVERA', 'ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS_THROUGH_AUTO_CLOSURE'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE case_cause_enum AS ENUM (
        'SOLUTION_ARCHITECTURE', 'DEPLOYMENT_ARCHITECTURE', 'USER_ERROR_CONFIGURATION',
        'USER_ERROR_PRODUCT_CONCEPT', 'USER_ERROR_RUNTIME',
        'USER_ERROR_RECOMMENDATION_BEST_PRACTICES', 'CUSTOMIZATION_LIMITATION',
        'CUSTOMIZATION_BUG', 'DOCUMENTATION_GAP', 'DOCUMENTATION_ERROR',
        'PRODUCT_LIMITATION', 'PRODUCT_BUG', 'PRODUCT_REGRESSION', 'PRODUCT_MIGRATION',
        'INFRASTRUCTURE_DATABASE', 'INFRASTRUCTURE_NETWORK', 'INFRASTRUCTURE_JDK',
        'INFRASTRUCTURE_LDAP', 'INFRASTRUCTURE_LOAD_BALANCER', 'INFRASTRUCTURE_IAAS',
        'INFRASTRUCTURE_EXTERNAL_PRODUCT', 'INFRASTRUCTURE_PROXY', 'INFRASTRUCTURE_OS',
        'INFRASTRUCTURE_OTHER', 'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Shared-primary-key extension of work_item: case.id IS work_item.id, not a
-- separately generated key. First table using this pattern; engagement and
-- service_request will follow the same shape.
CREATE TABLE IF NOT EXISTS "case" (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    severity case_severity_enum,
    issue_type case_issue_type_enum,
    state case_state_enum,
    current_escalation_level case_escalation_level_enum,
    is_escalated BOOLEAN,
    work_state case_work_state_enum,
    close_notes TEXT,
    resolution_code case_resolution_code_enum,
    cause case_cause_enum,
    closed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    resolved_on TIMESTAMPTZ,
    autoclosure_step VARCHAR(50),
    autoclosure_state_on TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_case_closed_by_user_id ON "case" (closed_by_user_id);
