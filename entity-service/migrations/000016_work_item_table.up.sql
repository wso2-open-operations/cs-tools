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
    CREATE TYPE work_item_type_enum AS ENUM ('CASE', 'ENGAGEMENT', 'SECURITY_REPORT_ANALYSIS', 'SERVICE_REQUEST', 'ANNOUNCEMENT', 'CHANGE_REQUEST', 'INCIDENT', 'PROBLEM');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS work_item (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(100) NOT NULL UNIQUE,
    -- Nullable: change_request work items have no wso2_id data.
    wso2_id VARCHAR(100) UNIQUE,
    subject VARCHAR(255) NOT NULL,
    type work_item_type_enum,
    account_id UUID REFERENCES account(id) ON DELETE SET NULL,
    project_id UUID REFERENCES project(id) ON DELETE SET NULL,
    deployment_id UUID REFERENCES deployment(id) ON DELETE SET NULL,
    deployed_product_id UUID REFERENCES deployed_product(id) ON DELETE SET NULL,
    contact_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    acknowledged_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    -- Self-referencing: a u_chat_conversation record lands as its own
    -- work_item row (type=CONVERSATION), so this just points at that row.
    conversation_id UUID REFERENCES work_item(id) ON DELETE SET NULL,
    best_case_eta DATE,
    most_likely_eta DATE,
    worst_case_eta DATE,
    eta_shared_on TIMESTAMPTZ,
    fix_issued_on TIMESTAMPTZ,
    workaround_provided_on TIMESTAMPTZ,
    workaround_provided_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    opened_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    business_duration INTERVAL,
    -- wso2_id is required for the work item types that map to a real CSM
    -- portal case identifier; change_request (and any other future type)
    -- has no such identifier and stays nullable.
    CONSTRAINT work_item_wso2_id_required_by_type CHECK (
        type NOT IN ('CASE', 'SERVICE_REQUEST', 'ANNOUNCEMENT', 'ENGAGEMENT', 'SECURITY_REPORT_ANALYSIS')
        OR wso2_id IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_work_item_account_id ON work_item (account_id);
CREATE INDEX IF NOT EXISTS idx_work_item_project_id ON work_item (project_id);
CREATE INDEX IF NOT EXISTS idx_work_item_deployment_id ON work_item (deployment_id);
CREATE INDEX IF NOT EXISTS idx_work_item_deployed_product_id ON work_item (deployed_product_id);
CREATE INDEX IF NOT EXISTS idx_work_item_contact_user_id ON work_item (contact_user_id);
CREATE INDEX IF NOT EXISTS idx_work_item_acknowledged_by_user_id ON work_item (acknowledged_by_user_id);
CREATE INDEX IF NOT EXISTS idx_work_item_conversation_id ON work_item (conversation_id);
CREATE INDEX IF NOT EXISTS idx_work_item_workaround_provided_by_user_id ON work_item (workaround_provided_by_user_id);
CREATE INDEX IF NOT EXISTS idx_work_item_opened_by_user_id ON work_item (opened_by_user_id);
