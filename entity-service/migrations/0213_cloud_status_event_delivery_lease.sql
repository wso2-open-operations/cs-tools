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

-- Claims on cloud status webhooks, so no event reaches the status page twice.
--
-- Two components now post these: csm-notification-service, the moment an
-- outage is written (outage.status_page_due), and csm-scheduled-tasks, which
-- retries. A duplicate on a public status page is not acceptable, so every
-- post is made under a claim that only one sender can hold:
--
--   claim_token        set when a row is reserved for, or attempted by, one
--                      sender. The reporter must present it (fencing).
--   claimed_until      how long that reservation or attempt is protected.
--   attempt_started_on set once a sender has won the right to post. From then
--                      on the row is NEVER claimed again: if no outcome is
--                      reported before claimed_until, nobody knows whether the
--                      dashboard got it, so it is left as "outcome unknown"
--                      and flagged rather than risk a second post.
--
-- A reservation that expires with no attempt started (the event was never
-- consumed) is safe to hand to the scheduled task: nothing was sent.
--
-- NULL on every existing row: they are pending exactly as before.
ALTER TABLE cloud_status_events ADD COLUMN IF NOT EXISTS claim_token UUID;
ALTER TABLE cloud_status_events ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ;
ALTER TABLE cloud_status_events ADD COLUMN IF NOT EXISTS attempt_started_on TIMESTAMPTZ;
