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
    CREATE TYPE outage_type_enum AS ENUM ('DEGRADATION', 'OUTAGE', 'PLANNED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- cmdb_ci is polymorphic on the ServiceNow side: it can point at either a
-- service or a service_offering row. Both FKs are nullable and exactly one
-- is populated per row (internal/transform's polymorphic_fk transform).
CREATE TABLE IF NOT EXISTS outage (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(255) UNIQUE,
    service_id UUID REFERENCES service(id) ON DELETE SET NULL,
    service_offering_id UUID REFERENCES service_offering(id) ON DELETE SET NULL,
    work_item_id UUID REFERENCES work_item(id) ON DELETE SET NULL,
    name VARCHAR(255),
    message VARCHAR(255),
    type outage_type_enum,
    start_on TIMESTAMPTZ,
    end_on TIMESTAMPTZ,
    duration INTERVAL,
    external_outage_communications TEXT,
    additional_outage_comments TEXT,
    internal_outage_communications TEXT
);
