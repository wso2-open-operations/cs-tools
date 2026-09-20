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
    CREATE TYPE product_manufacturer_enum AS ENUM ('WSO2_CLOUD', 'WSO2');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE product_category_enum AS ENUM ('SOFTWARE', 'SERVICE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE product_business_unit_enum AS ENUM ('INTEGRATION_SOFTWARE', 'INTEGRATION_CLOUD', 'IAM', 'CORPORATE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE product_unit_enum AS ENUM ('INTEGRATION', 'IOT', 'IAM', 'CHOREO', 'ASGARDEO', 'APIM', 'ANALYTICS');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS product (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    manufacturer product_manufacturer_enum NOT NULL,
    category product_category_enum NOT NULL,
    description TEXT,
    color VARCHAR(20),
    business_unit product_business_unit_enum,
    unit product_unit_enum,
    code VARCHAR(20),
    name VARCHAR(100) NOT NULL UNIQUE
);
