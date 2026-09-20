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

ALTER TABLE work_item ADD COLUMN IF NOT EXISTS assigned_to_id UUID REFERENCES "user"(id) ON DELETE SET NULL;

-- Self-referential (work_item -> work_item), deliberately SET NULL not CASCADE - a
-- self-referential cascade could ripple through an entire chain of child records, a much
-- less predictable blast radius than any other FK relationship in this schema.
ALTER TABLE work_item ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES work_item(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_work_item_assigned_to_id ON work_item (assigned_to_id);
CREATE INDEX IF NOT EXISTS idx_work_item_parent_id ON work_item (parent_id);
