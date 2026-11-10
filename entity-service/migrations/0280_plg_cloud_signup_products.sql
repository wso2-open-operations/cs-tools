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
-- A Cloud signup names its entry point as initiatingProduct;
-- plg_cloud_product_key maps each name to a product, several names per product
-- allowed. WSO2_CLOUD covers signups on the Cloud itself. Safe to re-run.

INSERT INTO plg_product (code, name, display_order, identity_source)
VALUES ('WSO2_CLOUD', 'WSO2 Cloud', 6, 'CLOUD_PLATFORM')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS plg_cloud_product_key (
    cloud_key  TEXT PRIMARY KEY CHECK (cloud_key = LOWER(cloud_key)),
    product_id UUID NOT NULL REFERENCES plg_product (id) ON DELETE CASCADE
);

COMMENT ON TABLE plg_cloud_product_key IS
    'WSO2 Cloud names (initiatingProduct, lower case) of each product.';

INSERT INTO plg_cloud_product_key (cloud_key, product_id)
SELECT k.key, p.id
FROM  (VALUES ('wso2cloud',            'WSO2_CLOUD'),
              ('core',                 'WSO2_CLOUD'),
              ('agent-manager',        'AGENT_PLATFORM'),
              ('api-platform',         'API_PLATFORM'),
              ('integration-platform', 'INTEGRATION_PLATFORM'),
              ('identity-platform',    'IAM')) AS k (key, code)
JOIN   plg_product p ON p.code = k.code
ON CONFLICT (cloud_key) DO UPDATE SET product_id = EXCLUDED.product_id;
