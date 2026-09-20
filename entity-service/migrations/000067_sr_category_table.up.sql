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

-- catalog is snake_upper of sc_catalog.title, dot-walked from
-- sc_category.sc_catalog.title (see sc_category.yaml).
DO $$ BEGIN
    CREATE TYPE sr_category_catalog_enum AS ENUM (
        'CONSUMER_SERVICE',
        'CHOREO_SERVICES',
        'CUSTOMER_SERVICE',
        'ASGARDEO_SERVICES',
        'RESOURCES',
        'WSO2_SERVICES',
        'SERVICE_CATALOG'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS sr_category (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN,
    catalog sr_category_catalog_enum
);
