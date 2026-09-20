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

-- classification values come from ServiceNow's Subscription Classifications
-- reference table (u_classification), confirmed as PDP, MS, PS, CL, PC.
DO $$ BEGIN
    CREATE TYPE sr_category_routing_rule_classification_enum AS ENUM ('PDP', 'MS', 'PS', 'CL', 'PC');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS sr_category_routing_rule (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    "order" INTEGER,
    product_unit product_unit_enum,
    classification sr_category_routing_rule_classification_enum,
    catalog_item_id UUID REFERENCES catalog_item(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sr_category_routing_rule_catalog_item_id ON sr_category_routing_rule (catalog_item_id);
