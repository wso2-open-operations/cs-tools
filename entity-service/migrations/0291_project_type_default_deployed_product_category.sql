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

-- The category a deployed product gets when it is created without one: the
-- default of its project's type. NULL means the type has no default and the
-- product stays uncategorised, as before. Entity-service's own column, like
-- 0130's entitlement columns on the same table; nothing else writes it.
--
-- Seeded by type name, only where it is still unset, so an edit made by hand
-- is kept and the file is safe to re-run. Development Support, Cloud Support -
-- Platformer, Platformer Subscription and Regular have no default (decided:
-- no products to go on).

SET lock_timeout = '5s';

ALTER TABLE project_type
    ADD COLUMN IF NOT EXISTS default_deployed_product_category deployed_product_category_enum;

UPDATE project_type pt
SET default_deployed_product_category = v.category::deployed_product_category_enum
FROM (VALUES
    ('Managed Cloud Subscription', 'MS'),
    ('Subscription', 'PS'),
    ('Cloud Support', 'CL'),
    ('Cloud Evaluation Support', 'CL'),
    ('Professional Services', 'PS'),
    ('Evaluation Subscription', 'PS'),
    ('Internal', 'PS')
) AS v(name, category)
WHERE pt.name = v.name
  AND pt.default_deployed_product_category IS NULL;

RESET lock_timeout;
