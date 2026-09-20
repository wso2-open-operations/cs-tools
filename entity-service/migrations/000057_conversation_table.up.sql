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

-- work_item type extension for CONVERSATION, same shared-primary-key pattern
-- as case/change_request (000018_case_table.up.sql, 000047_change_request_table.up.sql):
-- id IS work_item.id, ON DELETE CASCADE, no audit columns - those live on
-- work_item and are reachable via join.
DO $$ BEGIN
    CREATE TYPE conversation_state_enum AS ENUM (
        'OPEN', 'ACTIVE', 'RESOLVED', 'CONVERTED', 'ABANDONED', 'CLOSE'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS conversation (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    state conversation_state_enum
);
