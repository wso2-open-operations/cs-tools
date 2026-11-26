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

-- The alert processor (internal/alertprocessor) folds stored alerts into incidents from the alert.created events the consumer publishes; incident_id marks an alert done. Safe to run again.
-- Stop the alert consumer (ALERTING_VERSION=v1) before running this and start the new build after it: the old build writes high/default tiers, which this file retires, so its inserts would fail.

-- alert is FORCE ROW LEVEL SECURITY, so the tier update sees its rows only as the internal identity.
SELECT set_config('app.is_internal', 'true', false);

-- Tiers are named after the queue an alert arrived on. The check is swapped in one statement, so it is never absent; NOT VALID skips the scan under the exclusive lock, and VALIDATE checks existing rows without blocking reads or writes.
ALTER TABLE alert
    DROP CONSTRAINT IF EXISTS alert_tier_check,
    ADD CONSTRAINT alert_tier_check CHECK (tier IN ('priority', 'standard')) NOT VALID;
UPDATE alert SET tier = 'priority' WHERE tier = 'high';
UPDATE alert SET tier = 'standard' WHERE tier = 'default';
ALTER TABLE alert VALIDATE CONSTRAINT alert_tier_check;

SELECT set_config('app.is_internal', '', false);

-- The processor works from events and incident_id, so the claim columns go.
DROP INDEX IF EXISTS alert_unprocessed_idx;
DROP INDEX IF EXISTS alert_unprocessed_fingerprint_idx;
ALTER TABLE alert
    DROP COLUMN IF EXISTS claimed_by,
    DROP COLUMN IF EXISTS claim_expires_on,
    DROP COLUMN IF EXISTS processed_on;

-- The processor reads a fingerprint's alerts, newest first, to find the incident they fold into.
CREATE INDEX IF NOT EXISTS alert_fingerprint_idx ON alert (fingerprint, received_on) WHERE fingerprint IS NOT NULL;
