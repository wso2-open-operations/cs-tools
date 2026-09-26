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

-- Recreates sla_clocks exactly as 000042 + 000046 left it (schema only --
-- the Go code that wrote to it is gone, so this rollback restores the table
-- shape, not the feature).
CREATE TABLE IF NOT EXISTS sla_clocks (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id        TEXT NOT NULL,
  clock_type     TEXT NOT NULL,
  started_on     TIMESTAMPTZ NOT NULL,
  due_on         TIMESTAMPTZ NOT NULL,
  paused_on      TIMESTAMPTZ,
  reached_50_on  TIMESTAMPTZ,
  reached_75_on  TIMESTAMPTZ,
  reached_100_on TIMESTAMPTZ,
  created_on     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_on     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  case_number    TEXT,
  wso2_case_id   TEXT,
  case_title     TEXT,
  case_type      TEXT,
  product        TEXT,
  team           TEXT,
  priority       TEXT,
  state          TEXT,
  UNIQUE (case_id, clock_type)
);

CREATE INDEX IF NOT EXISTS idx_sla_clocks_case_id ON sla_clocks(case_id);
