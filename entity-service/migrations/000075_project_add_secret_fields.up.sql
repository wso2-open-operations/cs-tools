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

ALTER TABLE project ADD COLUMN IF NOT EXISTS product_consumption_primary_secret_key VARCHAR(128);
ALTER TABLE project ADD COLUMN IF NOT EXISTS product_consumption_secondary_secret_key VARCHAR(128);

-- u_license_secrets is a JSON-typed SN field (sys_dictionary: max length
-- 65,000), extracted via type: json_string rather than type: json/JSONB -
-- see customer_project.yaml's product_consumption_license_secrets field.
ALTER TABLE project ADD COLUMN IF NOT EXISTS product_consumption_license_secrets VARCHAR(65000);
