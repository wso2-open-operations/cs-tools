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
    CREATE TYPE service_commitment_type_enum AS ENUM (
        'AVAILABILITY', 'DELIVERY', 'MAINTENANCE_WINDOW', 'OTHER',
        'RECOVERY_POINT_OBJECTIVE', 'RECOVERY_TIME_OBJECTIVE', 'RESPONSE_TIME'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS service_commitment (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    description TEXT,
    type service_commitment_type_enum,
    precision INTEGER,
    percentage_avail NUMERIC,
    per VARCHAR(50),
    schedule VARCHAR(255),
    timezone VARCHAR(100),
    breach_penalty_amount NUMERIC(20,4)
);
