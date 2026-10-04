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

-- 0054's VARCHAR(36) assumed node_id is always a dashed UUID. Real data from
-- the product-consumption-tracking-integration source reports node ids as
-- 64-char hashes instead, so widen to the upstream string(128) size that same
-- migration's header comment already names as the real source width.
ALTER TABLE deployment_node ALTER COLUMN node_id TYPE VARCHAR(128);
ALTER TABLE deployment_information ALTER COLUMN node_id TYPE VARCHAR(128);
ALTER TABLE monthly_usage_summary ALTER COLUMN node_id TYPE VARCHAR(128);
