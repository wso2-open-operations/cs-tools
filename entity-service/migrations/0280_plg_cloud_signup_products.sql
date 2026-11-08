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

-- PLG: WSO2 Cloud signup products.
--
-- A Cloud signup names its entry point as initiatingProduct; cloud_product_key
-- maps it to a product, and WSO2_CLOUD covers signups on the Cloud itself.
-- Existing products keep their identity_source. Safe to re-run.

ALTER TABLE plg_product ADD COLUMN IF NOT EXISTS cloud_product_key TEXT;

DO $$ BEGIN
    ALTER TABLE plg_product
        ADD CONSTRAINT uq_plg_product_cloud_product_key UNIQUE (cloud_product_key);
EXCEPTION WHEN duplicate_object OR duplicate_table THEN NULL; END $$;

COMMENT ON COLUMN plg_product.cloud_product_key IS
    'WSO2 Cloud name for this product (initiatingProduct, lower case).';

INSERT INTO plg_product (code, name, display_order, identity_source)
VALUES ('WSO2_CLOUD', 'WSO2 Cloud', 6, 'CLOUD_PLATFORM')
ON CONFLICT (code) DO NOTHING;

UPDATE plg_product p
SET    cloud_product_key = k.key
FROM  (VALUES ('WSO2_CLOUD',           'wso2cloud'),
              ('AGENT_PLATFORM',       'agent-manager'),
              ('API_PLATFORM',         'api-platform'),
              ('INTEGRATION_PLATFORM', 'integration-platform'),
              ('IAM',                  'core')) AS k (code, key)
WHERE  p.code = k.code
  AND  p.cloud_product_key IS DISTINCT FROM k.key;
