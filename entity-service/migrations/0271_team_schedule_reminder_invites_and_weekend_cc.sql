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

-- Team Schedule: two rota reminder settings on the Rota Access tab.
--
-- 1. The calendar invitations of each rota reminder can be turned on or off
--    per rotation.
--
-- invite_rotations maps a rotation (the shift code of its window: CRE_MORNING,
-- CRE_EVENING, CRE_WEEKEND, CRE_WEEKEND_NIGHT) to whether its reminders carry
-- a calendar invitation. A rotation it does not name keeps them, so the empty
-- default changes nothing: every rotation sends its invitations, as before,
-- whenever invites_enabled is on. invites_enabled stays the switch for all of
-- them.
--
-- 2. weekend_copy_to: addresses copied (Cc) on the weekend summary email only,
--    beside copy_to, which is copied on every reminder. Empty by default, so
--    nothing changes until somebody adds an address.
--
-- Safe to re-run: each column, with its check, is added only when missing.

ALTER TABLE team_schedule_reminder_setting
    ADD COLUMN IF NOT EXISTS invite_rotations JSONB NOT NULL DEFAULT '{}'::jsonb
        CONSTRAINT team_schedule_reminder_setting_invite_rotations_check
            CHECK (jsonb_typeof(invite_rotations) = 'object');

ALTER TABLE team_schedule_reminder_setting
    ADD COLUMN IF NOT EXISTS weekend_copy_to TEXT[] NOT NULL DEFAULT '{}';
