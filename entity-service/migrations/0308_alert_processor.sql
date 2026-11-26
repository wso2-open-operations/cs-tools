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

-- The alert processor (internal/alertprocessor) groups stored alerts into incidents from alert.created events, so alert keeps only what was received. Safe to run again.
-- Stop the alert consumer (ALERTING_VERSION=v1) before running this and start the new build after it: the old build writes high/default tiers and the fingerprint column, which this file retires, so its inserts would fail.

-- alert is FORCE ROW LEVEL SECURITY, so the tier update sees its rows only as the internal identity.
SELECT set_config('app.is_internal', 'true', false);

-- Each check is swapped in one statement, so it is never absent; NOT VALID skips the scan under the exclusive lock, and VALIDATE checks existing rows without blocking reads or writes.

-- Tiers are named after the queue an alert arrived on.
ALTER TABLE alert
    DROP CONSTRAINT IF EXISTS alert_tier_check,
    ADD CONSTRAINT alert_tier_check CHECK (tier IN ('priority', 'standard')) NOT VALID;
UPDATE alert SET tier = 'priority' WHERE tier = 'high';
UPDATE alert SET tier = 'standard' WHERE tier = 'default';
ALTER TABLE alert VALIDATE CONSTRAINT alert_tier_check;

-- Grouping state lives in alert_group and delivery in alert_outbox, so the per-row fingerprint and claim columns go.
DROP INDEX IF EXISTS alert_unprocessed_idx;
DROP INDEX IF EXISTS alert_unprocessed_fingerprint_idx;
-- An alert row has a source and alert and no error; an error row is the message's only row, with an error and no alert.
ALTER TABLE alert
    DROP CONSTRAINT IF EXISTS alert_row_kind,
    DROP COLUMN IF EXISTS fingerprint,
    DROP COLUMN IF EXISTS claimed_by,
    DROP COLUMN IF EXISTS claim_expires_on,
    DROP COLUMN IF EXISTS processed_on,
    ADD CONSTRAINT alert_row_kind CHECK (
        (error_reason IS NULL AND error IS NULL AND source IS NOT NULL AND alert IS NOT NULL)
        OR (error_reason IS NOT NULL AND btrim(error_reason) <> '' AND alert IS NULL AND message_index = 0 AND incident_id IS NULL)
    ) NOT VALID;
ALTER TABLE alert VALIDATE CONSTRAINT alert_row_kind;

SELECT set_config('app.is_internal', '', false);

-- One row per stored alert still to be published as alert.created; written in the same statement as the alert.
CREATE TABLE IF NOT EXISTS alert_outbox (
    alert_id     TEXT        PRIMARY KEY REFERENCES alert(id) ON DELETE CASCADE,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_on TIMESTAMPTZ
);
-- The relay drains the oldest unpublished rows first.
CREATE INDEX IF NOT EXISTS alert_outbox_unpublished_idx ON alert_outbox (created_on) WHERE published_on IS NULL;

-- One row per fingerprint (sha256 of source|service|metric_name|environment|unique_identifier): the incident its alerts currently fold into.
CREATE TABLE IF NOT EXISTS alert_group (
    fingerprint   TEXT        PRIMARY KEY,
    incident_id   UUID        REFERENCES incident(id) ON DELETE SET NULL,
    first_seen_on TIMESTAMPTZ NOT NULL,
    last_seen_on  TIMESTAMPTZ NOT NULL,
    alert_count   INTEGER     NOT NULL DEFAULT 1 CHECK (alert_count > 0),
    updated_on    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE alert_group IS
    'Alert processor state: the incident each fingerprint folds into, and when that incident''s first alert arrived.';
