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

-- Public holidays added by hand on the Team Schedule.
--
-- An environment with no calendar-events connection (HOLIDAY_CALENDAR_*)
-- stores no holidays at all, so portal and rota admins can add them in the
-- portal (POST /team-schedule/public-holidays). source says where a row came
-- from: CALENDAR (the daily refresh, every row stored before this migration)
-- or MANUAL (added in the portal). The refresh replaces and deletes only
-- CALENDAR rows, so a manual day survives it, and a manual day the calendar
-- also has keeps its manual title. updated_by is who added or last changed a
-- manual row; NULL for the refresh.
--
-- Additive and safe to re-run: existing rows become CALENDAR.

ALTER TABLE team_schedule_public_holiday
    ADD COLUMN IF NOT EXISTS source VARCHAR(16) NOT NULL DEFAULT 'CALENDAR';

ALTER TABLE team_schedule_public_holiday
    ADD COLUMN IF NOT EXISTS updated_by VARCHAR(255);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'team_schedule_public_holiday_source_check'
           AND conrelid = 'team_schedule_public_holiday'::regclass
    ) THEN
        ALTER TABLE team_schedule_public_holiday
            ADD CONSTRAINT team_schedule_public_holiday_source_check
            CHECK (source IN ('CALENDAR', 'MANUAL'));
    END IF;
END $$;
