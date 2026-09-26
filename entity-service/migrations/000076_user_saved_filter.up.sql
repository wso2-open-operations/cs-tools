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

-- Named, per-user list filters for the CSM portal. A row is a label plus
-- the opaque query string the frontend already serializes for that list
-- (the URL stays the source of truth; this table does not parse filters).
-- list_key isolates cases / incidents / change_requests / problems so
-- filters never leak across lists. filter_position 0 is the first menu
-- entry. No ServiceNow equivalent — Postgres-only, like sla_clocks.
--
-- user_id is "user".id (JWT email → GetUserByEmail). Rows are removed when
-- the platform user is deleted.

CREATE TABLE IF NOT EXISTS user_saved_filter (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    list_key        VARCHAR(64) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    qs              TEXT NOT NULL,
    filter_position INT NOT NULL,
    created_on      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS user_saved_filter_user_list_name
    ON user_saved_filter (user_id, list_key, LOWER(name));

CREATE INDEX IF NOT EXISTS user_saved_filter_user_list_filter_position
    ON user_saved_filter (user_id, list_key, filter_position);
