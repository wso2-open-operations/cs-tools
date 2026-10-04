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

-- Kept in its own transaction: ALTER TYPE ... ADD VALUE can't run in the same
-- transaction as a statement that uses the new value. "Closed/Resolved by
-- Caller" is a real raw close_code (18 rows per the grouped report) meaning
-- the caller closed/resolved it themselves, distinct from WSO2 having solved
-- it - none of the existing 6 resolution codes fit.
ALTER TYPE incident_resolution_code_enum ADD VALUE IF NOT EXISTS 'RESOLVED_BY_CALLER';
ALTER TYPE incident_resolution_code_enum ADD VALUE IF NOT EXISTS 'DUPLICATE_ALERT';
