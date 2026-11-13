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

-- alert: one normalised SRE alert the Service Bus consumer stored, in the same shape as alert-core's alerts table. Safe to run again.
CREATE SEQUENCE IF NOT EXISTS alert_seq AS BIGINT START 1;

CREATE TABLE IF NOT EXISTS alert (
    -- ALT plus nine digits from alert_seq, the id format alert-ingestion writes, so ids sort in arrival order.
    id               TEXT        PRIMARY KEY,
    -- The gateway route the alert arrived on, such as azure or prometheus.
    source           TEXT        NOT NULL CHECK (btrim(source) <> ''),
    -- The canonical alert as JSON, with the same keys alert-ingestion writes.
    alert            JSONB       NOT NULL CHECK (jsonb_typeof(alert) = 'object'),
    -- sha256 of source|service|metric_name|environment|unique_identifier, the key alert-core folds incidents by.
    fingerprint      TEXT        NOT NULL,
    tier             TEXT        NOT NULL CHECK (tier IN ('high', 'default')),
    -- The Service Bus message and the alert's position in it; a redelivered message stores nothing new.
    message_id       TEXT        NOT NULL,
    message_index    INTEGER     NOT NULL CHECK (message_index >= 0),
    -- When the gateway accepted the request, which dedup windows are measured on.
    received_on      TIMESTAMPTZ NOT NULL,
    created_on       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Claim columns for the processor, which takes unprocessed rows with FOR UPDATE SKIP LOCKED.
    claimed_by       TEXT,
    claim_expires_on TIMESTAMPTZ,
    processed_on     TIMESTAMPTZ,
    CONSTRAINT alert_message_key UNIQUE (message_id, message_index)
) WITH (fillfactor = 85, autovacuum_vacuum_scale_factor = 0.05, autovacuum_vacuum_insert_scale_factor = 0.05);

-- The processor seeds a claim from the oldest unprocessed ids.
CREATE INDEX IF NOT EXISTS alert_unprocessed_idx ON alert (id) WHERE processed_on IS NULL;
-- The processor pulls the rest of a fingerprint into the same claim, so one incident's alerts fold together.
CREATE INDEX IF NOT EXISTS alert_unprocessed_fingerprint_idx ON alert (fingerprint) WHERE processed_on IS NULL;

COMMENT ON TABLE alert IS
    'SRE alerts the Service Bus consumer stored: one row per normalised alert, deduplicated by Service Bus message.';
