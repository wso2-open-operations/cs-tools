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

-- sla_policy.applies_to names the policy family a row belongs to. NULL (every
-- row today) keeps the existing behaviour: the case and incident resolvers
-- classify the row by its condition markers. A row tagged 'PRODUCT_ISSUE' is
-- selected by that tag alone, and case and incident resolution never see it.
-- No data is changed here.
ALTER TABLE sla_policy ADD COLUMN IF NOT EXISTS applies_to TEXT NULL;

ALTER TABLE sla_policy DROP CONSTRAINT IF EXISTS sla_policy_applies_to_check;
ALTER TABLE sla_policy ADD CONSTRAINT sla_policy_applies_to_check
    CHECK (applies_to IS NULL OR applies_to = 'PRODUCT_ISSUE');
