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

CREATE TABLE IF NOT EXISTS event_publish_failures (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type    TEXT NOT NULL,
  entity_id     TEXT NOT NULL,
  payload       JSONB NOT NULL,
  error         TEXT NOT NULL,
  created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_on   TIMESTAMPTZ
);

-- Index to optimize the common "show me the unresolved backlog" query —
-- resolved_on IS NULL, ordered the same way Search's query is (created_on
-- DESC, id) so Postgres can satisfy that ordering directly from the index
-- instead of sorting equal-timestamp rows itself.

CREATE INDEX IF NOT EXISTS idx_event_publish_failures_unresolved
  ON event_publish_failures(created_on DESC, id)
  WHERE resolved_on IS NULL;
