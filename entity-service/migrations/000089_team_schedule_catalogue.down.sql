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

-- Only the seeded catalogue rows are removed. Any window a lead added later
-- is left alone, and an assignment referencing a seeded window blocks the
-- delete (ON DELETE RESTRICT) rather than silently orphaning the roster.
DELETE FROM schedule_shift WHERE code IN (
    'CRE_MORNING', 'CRE_MORNING_OC', 'CRE_REGULAR', 'CRE_EVENING',
    'CRE_AMERICAS', 'CRE_WEEKEND',
    'SRE_TZ1_L1', 'SRE_TZ1', 'SRE_TZ2_L1', 'SRE_TZ2', 'SRE_TZ3',
    'SRE_WE_TZ1', 'SRE_WE_TZ2', 'SRE_REGULAR'
);

UPDATE schedule_zone SET weekend_zone_id = NULL;
DELETE FROM schedule_zone WHERE code IN ('TZ1', 'TZ2', 'TZ3');
