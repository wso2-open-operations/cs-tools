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

DO $$ BEGIN
    CREATE TYPE cloud_monitor_cloud_offering_enum AS ENUM (
        'ASGARDEO', 'BIJIRA', 'AGENT_MANAGER', 'CHOREO_EU', 'MOESIF', 'DEVANT', 'CHOREO'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE cloud_monitor_status_enum AS ENUM (
        'PARTIAL_OUTAGE', 'MAINTENANCE', 'DEGRADED', 'OPERATIONAL', 'MAJOR_OUTAGE'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS cloud_monitor (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    "group" VARCHAR(255),
    group_priority INTEGER,
    description TEXT,
    service_offering_id UUID REFERENCES service_offering(id) ON DELETE SET NULL,
    service_id UUID REFERENCES service(id) ON DELETE SET NULL,
    cloud_offering cloud_monitor_cloud_offering_enum,
    region VARCHAR(100),
    status cloud_monitor_status_enum,
    is_active BOOLEAN
);
