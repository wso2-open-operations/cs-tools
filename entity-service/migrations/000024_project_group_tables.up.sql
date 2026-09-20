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

CREATE TABLE IF NOT EXISTS project_group (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    "group" VARCHAR(255) NOT NULL UNIQUE
);

-- created_on/updated_on/created_by/updated_by are copied from the same
-- parent source record for every expanded row - ServiceNow has one audit
-- trail per bundle record, not per role pairing.
CREATE TABLE IF NOT EXISTS project_group_role (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    project_group_id UUID NOT NULL REFERENCES project_group(id) ON DELETE CASCADE,
    project_role_id UUID NOT NULL REFERENCES project_role(id) ON DELETE CASCADE,
    UNIQUE (project_group_id, project_role_id)
);

CREATE INDEX IF NOT EXISTS idx_project_group_role_project_group_id ON project_group_role (project_group_id);
CREATE INDEX IF NOT EXISTS idx_project_group_role_project_role_id ON project_group_role (project_role_id);
