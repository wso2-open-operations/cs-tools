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

-- Uniqueness moves from (name, unit) to (name, category, unit): a SOFTWARE
-- row and a SERVICE row sharing a name and unit are distinct products, not
-- duplicates, so category must be part of the key too.
ALTER TABLE product DROP CONSTRAINT IF EXISTS product_name_key;
ALTER TABLE product DROP CONSTRAINT IF EXISTS product_name_unit_key;
ALTER TABLE product DROP CONSTRAINT IF EXISTS product_name_category_unit_key;
ALTER TABLE product ADD CONSTRAINT product_name_category_unit_key UNIQUE (name, category, unit);
