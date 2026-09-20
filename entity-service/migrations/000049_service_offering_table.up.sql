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

-- Reuses service_status_enum, service_business_criticality_enum, service_consumer_type_enum,
-- and service_state_enum from 000048_service_table.up.sql: service_offering shares the
-- identical value sets for these fields.
CREATE TABLE IF NOT EXISTS service_offering (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    number VARCHAR(255),
    status service_status_enum,
    business_criticality service_business_criticality_enum,
    parent_id UUID REFERENCES service(id),
    consumer_type service_consumer_type_enum,
    state service_state_enum
);

CREATE INDEX IF NOT EXISTS idx_service_offering_parent_id ON service_offering (parent_id);
