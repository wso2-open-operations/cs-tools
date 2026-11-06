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

-- Team Schedule: the Annual/Casual leave chip reads "AL/CL", not "AL".
--
-- 0221 renamed the tag to "Annual/Casual leave", but its chip still read "AL",
-- which a reader scanning the roster takes for annual leave only. The short
-- code is display text (the tag is keyed by `code`, ANNUAL_LEAVE, and the
-- colour by `colour_token`, still AL), so nothing that reads the tag changes.
--
-- Data only, no schema change. Safe to re-run: it changes nothing once applied.

UPDATE team_schedule_absence_kind
   SET short_code = 'AL/CL',
       updated_on = now(),
       updated_by = 'migration:0249_team_schedule_annual_casual_short_code'
 WHERE code = 'ANNUAL_LEAVE'
   AND short_code IS DISTINCT FROM 'AL/CL';
