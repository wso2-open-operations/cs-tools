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

-- first_reported_by_task (the task a problem was first reported from) was
-- incorrectly feeding work_item.parent_id, a column meant only for
-- ServiceNow's generic "parent" reference (set uniformly across every
-- work_item type). Gets its own column instead.
ALTER TABLE problem ADD COLUMN IF NOT EXISTS origin_work_item_id UUID REFERENCES work_item(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_problem_origin_work_item_id ON problem (origin_work_item_id);
