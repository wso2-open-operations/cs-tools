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

-- "group" is a reserved word (like "user") and must stay double-quoted in
-- raw SQL. parent_id is intentionally left out of the initial mapping
-- version (see sys_user_group_all.yaml) - it's self-referential, and the
-- full_migration pages in sys_id order, not hierarchy order, so a child
-- row can be inserted before its parent exists and violate the FK. It's
-- added as a field_backfill (version bump) only once every group row
-- already exists, per the same table_name.
CREATE TABLE IF NOT EXISTS "group" (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    description TEXT,
    group_email VARCHAR(255),
    manager_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    parent_id UUID REFERENCES "group"(id) ON DELETE SET NULL,
    is_active BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_group_manager_id ON "group" (manager_id);
CREATE INDEX IF NOT EXISTS idx_group_parent_id ON "group" (parent_id);
