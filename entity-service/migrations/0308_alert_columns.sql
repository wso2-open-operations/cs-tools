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

-- The alert table's final shape for the consumer: tiers named after their queues, a UUID id next to each alert's ALT number, and every alert field in its own VARCHAR column. Safe to run again.
-- Stop the alert consumer (ALERTING_VERSION=v1) before running this and start the new build after it: the old build writes high/default tiers and the id and source columns, which this file retires and renames, so its inserts would fail. The columns the consumer stops writing (route, message_id, message_index and the rest) are dropped by 0309.

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

-- source held the gateway route the message arrived on. It moves aside to route, which nothing writes any more and 0309 drops, so source can be the monitoring source the alert names; the alert JSON column marks a table 0309 has not run on, after which source is already the monitoring source.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'alert' AND column_name = 'route')
        AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'alert' AND column_name = 'alert') THEN
        ALTER TABLE alert RENAME COLUMN source TO route;
        ALTER TABLE alert RENAME CONSTRAINT alert_source_check TO alert_route_check;
    END IF;
END $$;

-- Every alert field becomes a column, so alerts can be filtered by environment, service or severity without reading JSON. The consumer cuts a longer value to the column's length before storing it.
ALTER TABLE alert
    ADD COLUMN IF NOT EXISTS source            VARCHAR(255),
    ADD COLUMN IF NOT EXISTS service           VARCHAR(255),
    ADD COLUMN IF NOT EXISTS environment       VARCHAR(100),
    ADD COLUMN IF NOT EXISTS metric_name       VARCHAR(255),
    ADD COLUMN IF NOT EXISTS severity          VARCHAR(50),
    ADD COLUMN IF NOT EXISTS category          VARCHAR(100),
    ADD COLUMN IF NOT EXISTS unique_identifier VARCHAR(255),
    ADD COLUMN IF NOT EXISTS description       VARCHAR(4000),
    ADD COLUMN IF NOT EXISTS assignment_group  VARCHAR(255);

-- The steps up to the row check read columns 0309 drops (message_id, message_index, error_reason, the alert JSON), so they run only while the alert JSON column is still there.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'alert' AND column_name = 'alert') THEN
        RETURN;
    END IF;

    -- The consumer no longer writes the message's queue id and position (message_id, message_index) or the gateway route, so they may be empty until 0309 drops them, and the raw body is no longer tied to message_index.
    ALTER TABLE alert
        ALTER COLUMN message_id DROP NOT NULL,
        ALTER COLUMN message_index DROP NOT NULL,
        DROP CONSTRAINT IF EXISTS alert_raw_body_first_row;

    -- The consumer no longer writes the alert JSON, the fingerprint or error rows (a message it cannot normalise is dead-lettered), so the row check moves to the columns. The fingerprint is computed from source, service, metric_name, environment and unique_identifier, so they are never NULL on an alert ('' when empty). Error rows stored before this stay valid until 0309 removes them.
    ALTER TABLE alert
        DROP CONSTRAINT IF EXISTS alert_row_kind,
        ADD CONSTRAINT alert_row_kind CHECK (
            (error_reason IS NULL AND source IS NOT NULL AND service IS NOT NULL AND metric_name IS NOT NULL
                AND environment IS NOT NULL AND unique_identifier IS NOT NULL)
            OR (error_reason IS NOT NULL AND incident_id IS NULL)
        ) NOT VALID;

    -- Fills the columns of alerts stored before them from their JSON, cut to each column's length: a fingerprint field is '' when empty, any other field NULL.
    UPDATE alert SET
        source            = left(coalesce(alert.alert->>'source', ''), 255),
        service           = left(coalesce(alert.alert->>'service', ''), 255),
        environment       = left(coalesce(alert.alert->>'environment', ''), 100),
        metric_name       = left(coalesce(alert.alert->>'metric_name', ''), 255),
        severity          = left(nullif(alert.alert->>'severity', ''), 50),
        category          = left(nullif(alert.alert->>'category', ''), 100),
        unique_identifier = left(coalesce(alert.alert->>'unique_identifier', ''), 255),
        description       = left(nullif(alert.alert->>'description', ''), 4000),
        assignment_group  = left(nullif(alert.alert->>'assignment_group', ''), 255)
    WHERE alert.alert IS NOT NULL AND alert.unique_identifier IS NULL;

    ALTER TABLE alert VALIDATE CONSTRAINT alert_row_kind;
END $$;

-- VARCHAR rather than TEXT for the columns the consumer writes; also converts a column an earlier run of this file added as TEXT.
ALTER TABLE alert
    ALTER COLUMN number            TYPE VARCHAR(20),
    ALTER COLUMN tier              TYPE VARCHAR(20),
    ALTER COLUMN source            TYPE VARCHAR(255),
    ALTER COLUMN service           TYPE VARCHAR(255),
    ALTER COLUMN environment       TYPE VARCHAR(100),
    ALTER COLUMN metric_name       TYPE VARCHAR(255),
    ALTER COLUMN severity          TYPE VARCHAR(50),
    ALTER COLUMN category          TYPE VARCHAR(100),
    ALTER COLUMN unique_identifier TYPE VARCHAR(255),
    ALTER COLUMN description       TYPE VARCHAR(4000),
    ALTER COLUMN assignment_group  TYPE VARCHAR(255);

SELECT set_config('app.is_internal', '', false);

-- The portal lists alerts newest first, and finds them by environment or service.
CREATE INDEX IF NOT EXISTS alert_received_on_idx ON alert (received_on DESC);
CREATE INDEX IF NOT EXISTS alert_environment_idx ON alert (environment, received_on DESC);
