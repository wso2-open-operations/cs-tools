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
    CREATE TYPE deployed_product_lifecycle_stage_status_enum AS ENUM (
        'AVAILABLE', 'BUILD', 'BUY_OUT', 'CANCELLED', 'CHARTERED', 'DESIGN', 'DISPOSED',
        'DONATED', 'DRAFT', 'END_OF_SUPPORT', 'EXPIRED', 'IN_MAINTENANCE', 'IN_STOCK',
        'IN_TRANSIT', 'IN_USE', 'LEASE_RETURN', 'LEGAL_HOLD', 'LOST', 'OBSOLETE', 'ON_HOLD'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE deployed_product_lifecycle_stage_enum AS ENUM (
        'DEFECTIVE', 'DEPLOY', 'DESIGN', 'END_OF_LIFE', 'END_OF_OPERATION', 'IDEATION',
        'INVENTORY', 'MISSING', 'OPERATIONAL', 'PURCHASE', 'TO_BE_DETERMINED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE deployed_product_category_enum AS ENUM ('PDP', 'MS', 'PS', 'CL', 'PC');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS deployed_product (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    number VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255),
    description VARCHAR(255),
    active BOOLEAN,
    life_cycle_stage_status deployed_product_lifecycle_stage_status_enum,
    life_cycle_stage deployed_product_lifecycle_stage_enum,
    core_count INTEGER,
    tps_count NUMERIC,
    project_id UUID REFERENCES project(id) ON DELETE SET NULL,
    deployment_id UUID REFERENCES deployment(id) ON DELETE SET NULL,
    product_id UUID REFERENCES product(id) ON DELETE SET NULL,
    version_id UUID REFERENCES product_version(id) ON DELETE SET NULL,
    product_category deployed_product_category_enum,
    update_level_info JSONB
);

CREATE INDEX IF NOT EXISTS idx_deployed_product_project_id ON deployed_product (project_id);
CREATE INDEX IF NOT EXISTS idx_deployed_product_deployment_id ON deployed_product (deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployed_product_product_id ON deployed_product (product_id);
CREATE INDEX IF NOT EXISTS idx_deployed_product_version_id ON deployed_product (version_id);
