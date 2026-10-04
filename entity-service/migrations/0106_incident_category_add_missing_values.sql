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

-- Kept separate from statements that use these values: ALTER TYPE ... ADD
-- VALUE can't run in the same transaction as one that uses the new value.
-- Real raw incident.category values observed in production (grouped report,
-- 2026-09-29) with no matching enum entry. "Inquiry / Help" and the
-- "service_interuption" typo are NOT new values here - see
-- incident_details.yaml's category value_map, which maps both onto the
-- existing INQUIRY/SERVICE_INTERRUPTION values instead of duplicating them.
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'EC2_INSTANCE';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'APPLICATION';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'CONNECTIVITY';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'ELASTIC_LOAD_BALANCER';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'ERRORS';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'METRICS';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'MONITORING';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'PERFORMANCE';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'RDS_DATABASE';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'RESOURCE_EXHAUSTION';
ALTER TYPE incident_category_enum ADD VALUE IF NOT EXISTS 'TECHNICAL_FAULTS';
