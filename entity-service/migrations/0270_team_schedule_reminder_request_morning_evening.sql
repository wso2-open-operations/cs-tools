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

-- Team Schedule: "Send now" can ask for the Morning or the Evening reminder on
-- its own.
--
-- A rota reminder request's kind was 'weekday' (the next working day's Morning,
-- with its on-call, and Evening together) or 'weekend'. The Rota Access tab now
-- offers three: Morning (with its on-call), Evening and Weekend, so the kind
-- also takes 'morning' and 'evening'. 'weekday' stays allowed: requests already
-- stored keep their kind, and a portal not yet updated still sends it.
--
-- Safe to re-run: the check is dropped if present and added again.

ALTER TABLE team_schedule_reminder_request
    DROP CONSTRAINT IF EXISTS team_schedule_reminder_request_kind_check;

ALTER TABLE team_schedule_reminder_request
    ADD CONSTRAINT team_schedule_reminder_request_kind_check
        CHECK (kind IN ('weekday', 'morning', 'evening', 'weekend'));
