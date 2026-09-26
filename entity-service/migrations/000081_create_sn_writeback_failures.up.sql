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

CREATE TABLE IF NOT EXISTS sn_writeback_failures (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type   TEXT NOT NULL,
  entity_id     UUID NOT NULL,
  operation     TEXT NOT NULL,
  payload       JSONB NOT NULL,
  error         TEXT NOT NULL,
  created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index to optimize the common "show me the backlog for this entity" query --
-- newest first, same ordering as event_publish_failures' equivalent index.

CREATE INDEX IF NOT EXISTS idx_sn_writeback_failures_entity
  ON sn_writeback_failures(entity_type, entity_id, created_on DESC);
