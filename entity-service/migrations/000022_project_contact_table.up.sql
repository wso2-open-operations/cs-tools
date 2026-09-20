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
    CREATE TYPE project_contact_state_enum AS ENUM ('INVITED', 'REGISTERED', 'RE-INVITED', 'DEACTIVATED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS project_contact (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    state project_contact_state_enum,
    invitation_url TEXT,
    account_contact_id UUID NOT NULL REFERENCES account_contact(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_contact_account_contact_id ON project_contact (account_contact_id);
CREATE INDEX IF NOT EXISTS idx_project_contact_project_id ON project_contact (project_id);
