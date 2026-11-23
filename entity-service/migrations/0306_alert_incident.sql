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

-- alert.incident_id: the incident an alert became part of. Empty when the consumer stores the alert; the alert processor fills it once it creates or finds the incident. Safe to run again.
ALTER TABLE alert
    ADD COLUMN IF NOT EXISTS incident_id UUID REFERENCES incident(id) ON DELETE SET NULL;

-- Finds every alert of one incident, and keeps a deleted incident's SET NULL from scanning the table.
CREATE INDEX IF NOT EXISTS alert_incident_id_idx ON alert (incident_id) WHERE incident_id IS NOT NULL;

COMMENT ON COLUMN alert.incident_id IS
    'The incident this alert belongs to; NULL until the alert processor links it.';
