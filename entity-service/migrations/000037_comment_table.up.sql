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
    CREATE TYPE comment_type_enum AS ENUM ('APPROVAL_HISTORY', 'COMMENT', 'WORK_NOTE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- No updated_on/updated_by: sys_journal_field entries are append-only in ServiceNow, never
-- edited after creation, so there's nothing to populate those columns from.
CREATE TABLE IF NOT EXISTS comment (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    type comment_type_enum,
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    content TEXT
);

CREATE INDEX IF NOT EXISTS idx_comment_work_item_id ON comment (work_item_id);
