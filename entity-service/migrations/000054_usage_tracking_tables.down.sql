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

-- Children before the dimension they reference; the enum last, once the only
-- column using it is gone. Indexes go with their tables.
DROP TABLE IF EXISTS product_usage_map;
DROP TABLE IF EXISTS project_daily_summary;
DROP TABLE IF EXISTS monthly_usage_count;
DROP TABLE IF EXISTS daily_usage_summary;
DROP TABLE IF EXISTS usage_count;
DROP TABLE IF EXISTS deployment_information;
DROP TABLE IF EXISTS deployment_node;

DROP TYPE IF EXISTS usage_data_source_enum;
