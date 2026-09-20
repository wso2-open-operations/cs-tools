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

-- product_id was already being written to by the u_wso2_product mapping
-- field but the column was never added to work_item.
ALTER TABLE work_item ADD COLUMN IF NOT EXISTS product_id UUID REFERENCES product(id) ON DELETE SET NULL;
ALTER TABLE work_item ADD COLUMN IF NOT EXISTS product_version_id UUID REFERENCES product_version(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_work_item_product_id ON work_item (product_id);
CREATE INDEX IF NOT EXISTS idx_work_item_product_version_id ON work_item (product_version_id);
