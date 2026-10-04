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

-- The same product/version can legitimately appear more than once with a
-- different deployment_profile (e.g. "All In One" vs a split profile), so
-- deployment_profile joins the uniqueness instead of just (product_id, version).
ALTER TABLE product_version DROP CONSTRAINT IF EXISTS product_version_product_id_version_key;
ALTER TABLE product_version DROP CONSTRAINT IF EXISTS product_version_product_id_version_deployment_profile_key;
ALTER TABLE product_version ADD CONSTRAINT product_version_product_id_version_deployment_profile_key
    UNIQUE (product_id, version, deployment_profile);
