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

-- Team Schedule: a separate Cc list for the rota reminders' calendar
-- invitations, set on the Rota Access tab.
--
-- invite_copy_to: addresses copied (Cc) on the emails that carry a calendar
-- invitation or its cancellation, and on no other. copy_to now covers the
-- reminder emails only (weekday reminders and the weekend summary), without
-- the invitation attached. Empty by default: nobody is moved into it.
--
-- Safe to re-run: the column is added only when missing.

ALTER TABLE team_schedule_reminder_setting
    ADD COLUMN IF NOT EXISTS invite_copy_to TEXT[] NOT NULL DEFAULT '{}';
