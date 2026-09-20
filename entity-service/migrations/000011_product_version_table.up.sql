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
    CREATE TYPE product_version_support_status_enum AS ENUM ('AVAILABLE', 'DEPRECATED', 'DISCONTINUED', 'EXTENDED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS product_version (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    version VARCHAR(20) NOT NULL,
    product_id UUID NOT NULL REFERENCES product(id),
    serial_number VARCHAR(255),
    deployment_profile VARCHAR(255),
    current_support_status product_version_support_status_enum,
    earliest_possible_support_eol_date DATE,
    support_eol_date DATE,
    release_date DATE,
    UNIQUE (product_id, version)
);

CREATE INDEX IF NOT EXISTS idx_product_version_product_id ON product_version (product_id);
