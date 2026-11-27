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

-- The alert table's final shape for the consumer: tiers named after their queues, a UUID id next to each alert's ALT number, and every alert field in its own column. Safe to run again.
-- Stop the alert consumer (ALERTING_VERSION=v1) before running this and start the new build after it: the old build writes high/default tiers and the id and source columns, which this file retires and renames, so its inserts would fail.

-- alert is FORCE ROW LEVEL SECURITY, so the updates below see its rows only as the internal identity.
SELECT set_config('app.is_internal', 'true', false);

-- Tiers are named after the queue an alert arrived on. The check is swapped in one statement, so it is never absent; NOT VALID skips the scan under the exclusive lock, and VALIDATE checks existing rows without blocking reads or writes.
ALTER TABLE alert
    DROP CONSTRAINT IF EXISTS alert_tier_check,
    ADD CONSTRAINT alert_tier_check CHECK (tier IN ('priority', 'standard')) NOT VALID;
UPDATE alert SET tier = 'priority' WHERE tier = 'high';
UPDATE alert SET tier = 'standard' WHERE tier = 'default';
ALTER TABLE alert VALIDATE CONSTRAINT alert_tier_check;

-- Like incidents, an alert has a UUID id and a number people quote: the ALT id moves to number, and id becomes a UUID primary key.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'alert' AND column_name = 'number') THEN
        ALTER TABLE alert RENAME COLUMN id TO number;
        ALTER TABLE alert DROP CONSTRAINT alert_pkey;
        ALTER TABLE alert ADD CONSTRAINT alert_number_key UNIQUE (number);
        ALTER TABLE alert ADD COLUMN id UUID NOT NULL DEFAULT uuidv7();
        ALTER TABLE alert ADD CONSTRAINT alert_pkey PRIMARY KEY (id);
    END IF;
END $$;

-- Every alert field becomes a column, so alerts can be filtered by environment, service or severity without reading JSON. source was the gateway route; it is now route, and source is the monitoring source the alert names.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'alert' AND column_name = 'route') THEN
        ALTER TABLE alert RENAME COLUMN source TO route;
        ALTER TABLE alert RENAME CONSTRAINT alert_source_check TO alert_route_check;
    END IF;
END $$;

ALTER TABLE alert
    ADD COLUMN IF NOT EXISTS source            TEXT,
    ADD COLUMN IF NOT EXISTS service           TEXT,
    ADD COLUMN IF NOT EXISTS environment       TEXT,
    ADD COLUMN IF NOT EXISTS metric_name       TEXT,
    ADD COLUMN IF NOT EXISTS severity          TEXT,
    ADD COLUMN IF NOT EXISTS category          TEXT,
    ADD COLUMN IF NOT EXISTS unique_identifier TEXT,
    ADD COLUMN IF NOT EXISTS description       TEXT,
    ADD COLUMN IF NOT EXISTS assignment_group  TEXT,
    ADD COLUMN IF NOT EXISTS source_topic      TEXT,
    ADD COLUMN IF NOT EXISTS source_account    TEXT;

-- The consumer no longer writes the alert JSON, the fingerprint or error rows (a message it cannot normalise is dead-lettered), so the row check moves to the columns. The fingerprint is computed from source, service, metric_name, environment and unique_identifier, so they are never NULL on an alert ('' when empty). Error rows stored before this stay valid until 0309 removes them.
ALTER TABLE alert
    DROP CONSTRAINT IF EXISTS alert_row_kind,
    ADD CONSTRAINT alert_row_kind CHECK (
        (error_reason IS NULL AND route IS NOT NULL AND source IS NOT NULL AND service IS NOT NULL AND metric_name IS NOT NULL
            AND environment IS NOT NULL AND unique_identifier IS NOT NULL)
        OR (error_reason IS NOT NULL AND incident_id IS NULL)
    ) NOT VALID;

-- Fills the columns of alerts stored before them from their JSON: a fingerprint field is '' when empty, any other field NULL.
UPDATE alert SET
    source            = coalesce(alert->>'source', ''),
    service           = coalesce(alert->>'service', ''),
    environment       = coalesce(alert->>'environment', ''),
    metric_name       = coalesce(alert->>'metric_name', ''),
    severity          = nullif(alert->>'severity', ''),
    category          = nullif(alert->>'category', ''),
    unique_identifier = coalesce(alert->>'unique_identifier', ''),
    description       = nullif(alert->>'description', ''),
    assignment_group  = nullif(alert->>'assignment_group', ''),
    source_topic      = nullif(alert->>'source_topic', ''),
    source_account    = nullif(alert->>'source_account', '')
WHERE alert IS NOT NULL AND unique_identifier IS NULL;

ALTER TABLE alert VALIDATE CONSTRAINT alert_row_kind;

SELECT set_config('app.is_internal', '', false);

-- The portal lists alerts newest first, and finds them by environment or service.
CREATE INDEX IF NOT EXISTS alert_received_on_idx ON alert (received_on DESC);
CREATE INDEX IF NOT EXISTS alert_environment_idx ON alert (environment, received_on DESC);
