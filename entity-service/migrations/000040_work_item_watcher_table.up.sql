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

-- Junction table fanned out from task.watch_list via expand_list, same shape
-- as project_group_role: one row per (work_item, watching user) pair.
-- Deliberately no created_on/updated_on/created_by/updated_by, unlike that
-- table and time_card_approver: watch_list is a bare list of user
-- references on the source record, with no per-entry audit trail to copy
-- down the way a bundle record's own fields are for those two.
CREATE TABLE IF NOT EXISTS work_item_watcher (
    id UUID PRIMARY KEY,
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    UNIQUE (work_item_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_work_item_watcher_work_item_id ON work_item_watcher (work_item_id);
CREATE INDEX IF NOT EXISTS idx_work_item_watcher_user_id ON work_item_watcher (user_id);
