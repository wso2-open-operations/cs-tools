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

-- Backfills the two FKs skipped in 000058_incident_table.up.sql: parent_incident_id
-- (self-referencing) and problem_id, both left out because problem didn't
-- exist yet. Now that problem (000059) is in place, both can be added.
ALTER TABLE incident
    ADD COLUMN IF NOT EXISTS parent_incident_id UUID REFERENCES incident(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS problem_id UUID REFERENCES problem(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_incident_parent_incident_id ON incident (parent_incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_problem_id ON incident (problem_id);
