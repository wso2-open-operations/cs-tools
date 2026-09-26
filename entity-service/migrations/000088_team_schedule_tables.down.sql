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

DROP TABLE IF EXISTS schedule_absence;
DROP TABLE IF EXISTS schedule_assignment;
DROP TABLE IF EXISTS schedule_shift;
DROP TABLE IF EXISTS schedule_zone;

DROP TYPE IF EXISTS schedule_absence_kind_enum;
DROP TYPE IF EXISTS schedule_source_enum;
DROP TYPE IF EXISTS schedule_day_scope_enum;
DROP TYPE IF EXISTS schedule_tier_enum;
DROP TYPE IF EXISTS schedule_shift_family_enum;
