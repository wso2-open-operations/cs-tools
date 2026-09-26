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

ALTER TABLE project_type
    DROP COLUMN IF EXISTS has_service_request_write_access,
    DROP COLUMN IF EXISTS has_service_request_read_access,
    DROP COLUMN IF EXISTS has_change_request_read_access,
    DROP COLUMN IF EXISTS has_sra_write_access,
    DROP COLUMN IF EXISTS has_sra_read_access,
    DROP COLUMN IF EXISTS has_engagements_read_access,
    DROP COLUMN IF EXISTS has_updates_read_access,
    DROP COLUMN IF EXISTS has_deployment_write_access,
    DROP COLUMN IF EXISTS has_deployment_read_access,
    DROP COLUMN IF EXISTS has_time_logs_read_access,
    DROP COLUMN IF EXISTS has_component_analysis_read_access,
    DROP COLUMN IF EXISTS has_usage_metrics_read_access,
    DROP COLUMN IF EXISTS accepted_severity_values,
    DROP COLUMN IF EXISTS default_case_product_categories,
    DROP COLUMN IF EXISTS sr_product_categories;
