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

-- Standalone entity table (own id/audit columns), same shape as
-- communication_plan (000062_communication_plan_table.up.sql), plus a
-- source_incident_id FK.
CREATE TABLE IF NOT EXISTS incident_alert (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(255) NOT NULL UNIQUE,
    opened_on TIMESTAMPTZ,
    opened_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    assigned_to_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    source_id UUID REFERENCES work_item(id) ON DELETE SET NULL,
    source_incident_id UUID REFERENCES incident(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_incident_alert_opened_by_id ON incident_alert (opened_by_id);
CREATE INDEX IF NOT EXISTS idx_incident_alert_assigned_to_id ON incident_alert (assigned_to_id);
CREATE INDEX IF NOT EXISTS idx_incident_alert_source_id ON incident_alert (source_id);
CREATE INDEX IF NOT EXISTS idx_incident_alert_source_incident_id ON incident_alert (source_incident_id);
