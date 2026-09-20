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
    CREATE TYPE service_status_enum AS ENUM (
        'BUILD_TEST_RELEASE', 'OBSOLETE', 'APPROVED', 'OPERATIONAL', 'REQUIREMENTS',
        'CHARTERED', 'RETIRING', 'DEFINITION', 'DESIGN', 'RETIRED', 'ANALYSIS', 'DEVELOPMENT'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE service_business_criticality_enum AS ENUM (
        'MOST_CRITICAL', 'SOMEWHAT_CRITICAL', 'LESS_CRITICAL', 'NOT_CRITICAL'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE service_consumer_type_enum AS ENUM ('BOTH', 'INTERNAL', 'EXTERNAL');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE service_state_enum AS ENUM ('PUBLISHED', 'DRAFT', 'PUBLISHING');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS service (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    status service_status_enum,
    number VARCHAR(100) NOT NULL,
    business_criticality service_business_criticality_enum,
    consumer_type service_consumer_type_enum,
    state service_state_enum,
    subcategory VARCHAR(100),
    category VARCHAR(100)
);
