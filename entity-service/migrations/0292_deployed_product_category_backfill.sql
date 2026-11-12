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

-- DATA FIX: gives every deployed product that has no category the default of
-- its project's type (migration 0291), the category a create without one now
-- gets. Only a NULL category is filled: a category someone set is never
-- changed. A product whose project cannot be found, or whose project's type
-- has no default, is left uncategorised.
--
-- The project is deployed_product.project_id, else the project of its
-- deployment: project_id is NULL on some synced rows, and the row-level
-- security policy reads the project the same way (migration 100017).
--
-- Safe to re-run: a second run finds no NULL category it can fill.
--
-- deployed_product is row-level-secured with FORCE (100017): a migrating role
-- that owns the table without BYPASSRLS would see none of the rows, so the
-- session is flagged internal for the update and cleared again afterwards, as
-- 0193 does. (Session-level, not LOCAL: a migration is applied with plain
-- autocommit, where a LOCAL setting would be gone before the UPDATE runs.)

SET lock_timeout = '5s';

SELECT set_config('app.is_internal', 'true', false);

UPDATE deployed_product dp
SET product_category = pt.default_deployed_product_category,
    updated_on = NOW(),
    updated_by = 'migration:0292_deployed_product_category_backfill'
FROM project p
JOIN project_type pt ON pt.id = p.project_type_id
WHERE dp.product_category IS NULL
  AND pt.default_deployed_product_category IS NOT NULL
  AND p.id = COALESCE(dp.project_id,
                      (SELECT d.project_id FROM deployment d WHERE d.id = dp.deployment_id));

SELECT set_config('app.is_internal', '', false);

RESET lock_timeout;
