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

-- Mirror ServiceNow's Skills Management plugin tables (cmn_skill*,
-- sys_user_has_skill, sys_user_skill_history) - see configs/mappings/
-- cmn_skill*.yaml, sys_user_has_skill.yaml, sys_user_skill_history.yaml.
CREATE TABLE IF NOT EXISTS skill_level_type (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    description TEXT
);

CREATE TABLE IF NOT EXISTS skill_level (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    value INTEGER,
    description TEXT,
    color VARCHAR(50),
    level_type_id UUID NOT NULL REFERENCES skill_level_type(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_level_level_type_id ON skill_level (level_type_id);

-- parent is nullable: root categories have no parent.
CREATE TABLE IF NOT EXISTS skill_category (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(100),
    category_path VARCHAR(500),
    is_lowest_category BOOLEAN NOT NULL DEFAULT false,
    parent_id UUID REFERENCES skill_category(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_category_parent_id ON skill_category (parent_id);

CREATE TABLE IF NOT EXISTS skill (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    keywords VARCHAR(500),
    level_type_id UUID REFERENCES skill_level_type(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_level_type_id ON skill (level_type_id);

CREATE TABLE IF NOT EXISTS skill_category_link (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    skill_id UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES skill_category(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_skill_category_link_skill_id ON skill_category_link (skill_id);
CREATE INDEX IF NOT EXISTS idx_skill_category_link_category_id ON skill_category_link (category_id);

-- inherited_from_group_id deliberately has no FK: this table is migrated
-- unscoped (every WSO2 user), so it can point at any SN group, most of
-- which are never migrated into team - see sys_user_has_skill.yaml.
CREATE TABLE IF NOT EXISTS user_skill (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    skill_level_id UUID REFERENCES skill_level(id) ON DELETE CASCADE,
    active BOOLEAN NOT NULL DEFAULT true,
    inherited BOOLEAN NOT NULL DEFAULT false,
    inherited_from_group_id UUID
);

CREATE INDEX IF NOT EXISTS idx_user_skill_user_id ON user_skill (user_id);
CREATE INDEX IF NOT EXISTS idx_user_skill_skill_id ON user_skill (skill_id);

CREATE TABLE IF NOT EXISTS user_skill_history (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    level_from_id UUID REFERENCES skill_level(id) ON DELETE CASCADE,
    level_to_id UUID REFERENCES skill_level(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_skill_history_user_id ON user_skill_history (user_id);
CREATE INDEX IF NOT EXISTS idx_user_skill_history_skill_id ON user_skill_history (skill_id);
