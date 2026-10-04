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

-- The two cmdb_ci_outage columns that drive ServiceNow's internal-stakeholder
-- outage notification, neither of which was mirrored.
--
-- `Internal Stakeholders Email Notification - Outage Communication` is a state
-- machine, not a simple notifier: it triggers on u_notify_internal_stakeholders
-- being true, and it reads AND WRITES u_internal_notification_phase to decide
-- which of three emails to send -- none -> declared -> resolved. Without both
-- columns in Postgres, a port cannot answer "should this outage notify?" or
-- "has it been declared yet?", so it would re-send the declaration email on
-- every update forever.
--
-- Both are populated on all 635 outage rows on the source instance, so neither
-- is a speculative column.

DO $$ BEGIN
    -- Values are ServiceNow's own choice list on u_internal_notification_phase
    -- (none | declared | resolved), upper-cased to match every other enum in
    -- this schema. NONE is a real state, not an absence: it is what an outage
    -- carries before the declaration email has gone out, and it is the value
    -- 634 of the 635 current rows hold.
    CREATE TYPE outage_internal_notification_phase_enum AS ENUM (
        'NONE', 'DECLARED', 'RESOLVED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE outage
    ADD COLUMN IF NOT EXISTS notify_internal_stakeholders BOOLEAN,
    ADD COLUMN IF NOT EXISTS internal_notification_phase  outage_internal_notification_phase_enum;

-- The notification port's only selective read is "outages that notify, whose
-- phase has not reached resolved". Nothing else filters on these columns.
CREATE INDEX IF NOT EXISTS idx_outage_internal_notification
    ON outage (notify_internal_stakeholders, internal_notification_phase)
    WHERE notify_internal_stakeholders IS TRUE;
