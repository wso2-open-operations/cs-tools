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

CREATE TABLE IF NOT EXISTS catalog_item_category (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    catalog_item_id UUID NOT NULL REFERENCES catalog_item(id) ON DELETE CASCADE,
    sr_category_id UUID NOT NULL REFERENCES sr_category(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_catalog_item_category_catalog_item_id ON catalog_item_category (catalog_item_id);
CREATE INDEX IF NOT EXISTS idx_catalog_item_category_sr_category_id ON catalog_item_category (sr_category_id);
