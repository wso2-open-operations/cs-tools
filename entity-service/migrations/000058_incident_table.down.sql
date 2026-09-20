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

DROP TABLE IF EXISTS incident;
DROP TABLE IF EXISTS incident_subcategory;
DROP TYPE IF EXISTS incident_priority_enum;
DROP TYPE IF EXISTS incident_state_enum;
DROP TYPE IF EXISTS incident_category_enum;
DROP TYPE IF EXISTS incident_impact_enum;
DROP TYPE IF EXISTS incident_urgency_enum;
DROP TYPE IF EXISTS incident_contact_type_enum;
DROP TYPE IF EXISTS incident_resolution_code_enum;
