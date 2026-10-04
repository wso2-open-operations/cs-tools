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

-- Maps a product name to the GitHub repository an internal issue is filed in.
-- One row per product. Many products may share one repository.
-- No foreign key to product: staging cases are read from ServiceNow, while
-- this table stays in Postgres.

CREATE TABLE IF NOT EXISTS product_repo_mapping (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(255) NOT NULL,
    updated_by    VARCHAR(255) NOT NULL,
    product_name  VARCHAR(255) NOT NULL,
    abbreviation  VARCHAR(64),
    owner         VARCHAR(255) NOT NULL,
    repository    VARCHAR(255) NOT NULL,
    github_label  VARCHAR(255) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT uq_product_repo_mapping_product_name UNIQUE (product_name),
    CONSTRAINT uq_product_repo_mapping_abbreviation UNIQUE (abbreviation)
);

-- Case-insensitive uniqueness so lookups by name/abbreviation are unambiguous.
CREATE UNIQUE INDEX IF NOT EXISTS uq_product_repo_mapping_product_name_ci
    ON product_repo_mapping (LOWER(product_name));

CREATE UNIQUE INDEX IF NOT EXISTS uq_product_repo_mapping_abbreviation_ci
    ON product_repo_mapping (LOWER(abbreviation));

CREATE INDEX IF NOT EXISTS idx_product_repo_mapping_active
    ON product_repo_mapping (is_active);
