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

-- tag / work_item_tag mirror ServiceNow's OOB label / label_entry tables
-- (label_entry is polymorphic: table + table_key identify the tagged row on
-- any table). Scoped here to work_item, the only tagged entity currently
-- synced - see label_entry.yaml's source_filter.
CREATE TABLE IF NOT EXISTS tag (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS work_item_tag (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tag(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_work_item_tag_work_item_id ON work_item_tag (work_item_id);
CREATE INDEX IF NOT EXISTS idx_work_item_tag_tag_id ON work_item_tag (tag_id);
