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

CREATE TABLE IF NOT EXISTS account (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    number VARCHAR(100) NOT NULL UNIQUE,
    sf_id VARCHAR(100) NOT NULL UNIQUE,
    activation_date DATE,
    deactivation_date DATE,
    phone VARCHAR(20),
    smart_knowledge_base_suggestions_enabled BOOLEAN,
    ai_gen_response_enabled BOOLEAN,
    classification VARCHAR(255),
    customer_success_manager_id UUID REFERENCES "user"(id),
    technical_owner_id UUID REFERENCES "user"(id),
    secondary_technical_owner_id UUID REFERENCES "user"(id),
    account_manager_id UUID REFERENCES "user"(id),
    renewal_account_manager_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    country VARCHAR(100),
    region VARCHAR(100),
    state_province VARCHAR(100),
    postal_code VARCHAR(20),
    city VARCHAR(100),
    street VARCHAR(255),
    sub_industry VARCHAR(255),
    industry VARCHAR(255),
    sub_region VARCHAR(100),
    drive_location TEXT,
    naics_industry VARCHAR(100),
    global_pod VARCHAR(100),
    sales_region VARCHAR(100),
    lost_reason VARCHAR(255),
    lost_reason_category VARCHAR(255),
    lost_date DATE,
    account_vertical VARCHAR(255),
    life_cycle VARCHAR(50),
    sync_time_stamp TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_account_customer_success_manager ON account (customer_success_manager_id);
CREATE INDEX IF NOT EXISTS idx_account_technical_owner ON account (technical_owner_id);
CREATE INDEX IF NOT EXISTS idx_account_secondary_technical_owner ON account (secondary_technical_owner_id);
CREATE INDEX IF NOT EXISTS idx_account_account_manager ON account (account_manager_id);
CREATE INDEX IF NOT EXISTS idx_account_renewal_account_manager ON account (renewal_account_manager_id);
