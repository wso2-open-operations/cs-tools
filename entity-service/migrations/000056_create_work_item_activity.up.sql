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

-- old_value/new_value are VARCHAR since each row can hold one of several
-- enum label sets depending on field_name.
CREATE TABLE IF NOT EXISTS work_item_activity (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    field_name VARCHAR(255),
    old_value VARCHAR(255),
    new_value VARCHAR(255),
    user_email VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_work_item_activity_work_item_id ON work_item_activity (work_item_id);
