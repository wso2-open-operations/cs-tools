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
    CREATE TYPE service_availability_type_enum AS ENUM (
        'ANNUALLY', 'DAILY', 'LAST_12_MONTHS', 'LAST_1_DAYS', 'LAST_30_DAYS',
        'LAST_7_DAYS', 'LAST_90_DAYS', 'MONTHLY', 'WEEKLY'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS service_availability (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    type service_availability_type_enum,
    service_offering_id UUID REFERENCES service_offering(id) ON DELETE SET NULL,
    service_commitment_id UUID REFERENCES service_commitment(id) ON DELETE SET NULL,
    start_on TIMESTAMPTZ,
    end_on TIMESTAMPTZ,
    time_zone VARCHAR(255),
    absolute_downtime_duration INTERVAL,
    scheduled_downtime_duration INTERVAL,
    absolute_count INTEGER,
    scheduled_count INTEGER,
    absolute_availability NUMERIC,
    scheduled_availability NUMERIC,
    committed_uptime_duration INTERVAL,
    mtbf_duration INTERVAL,
    mtrs_duration INTERVAL,
    is_commitment_met BOOLEAN,
    allowed_downtime_duration INTERVAL
);
