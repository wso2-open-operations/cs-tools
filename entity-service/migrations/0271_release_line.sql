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

-- release_line: a product release line (e.g. APIM-4.x) and its support
-- status, published on the Support Matrix. A version belongs to at most one
-- line. The legacy Product API modelled this as many-to-many, but no version
-- in its data is linked to two different lines.
--
-- legacy_id keeps the legacy numeric release_line_id so the compatible
-- /release-line feed can return it. Entity-service owns this table; it is not
-- a ServiceNow sync table. No row-level security.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS release_line (
    id          UUID         PRIMARY KEY DEFAULT uuidv7(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    start_date  DATE,
    status      VARCHAR(50),
    legacy_id   BIGINT       UNIQUE,
    retired_on  TIMESTAMPTZ,
    retired_by  VARCHAR(255),
    created_on  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by  VARCHAR(255) NOT NULL,
    updated_on  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by  VARCHAR(255) NOT NULL
);

COMMENT ON TABLE release_line IS
    'Product release lines (e.g. APIM-4.x) published on the Support Matrix. A product version belongs to at most one.';

ALTER TABLE product_version
    ADD COLUMN IF NOT EXISTS release_line_id UUID REFERENCES release_line (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_product_version_release_line_id
    ON product_version (release_line_id);

COMMIT;
