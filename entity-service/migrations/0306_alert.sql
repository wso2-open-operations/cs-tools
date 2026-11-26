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

-- alert: every SRE alert message the alert consumer received, as normalised alerts or, when it could not be normalised, the error. Safe to run again.
CREATE SEQUENCE IF NOT EXISTS alert_seq AS BIGINT START 1;

CREATE TABLE IF NOT EXISTS alert (
    -- ALT plus nine digits from alert_seq, the id format alert-ingestion writes, so ids sort in arrival order.
    id               TEXT        PRIMARY KEY,
    -- The gateway route the alert arrived on, such as azure or prometheus; NULL only on an error row whose message named no source.
    source           TEXT        CHECK (btrim(source) <> ''),
    -- The canonical alert as JSON, with the same keys alert-ingestion writes; NULL on an error row.
    alert            JSONB       CHECK (jsonb_typeof(alert) = 'object'),
    -- sha256 of source|service|metric_name|environment|unique_identifier, the key alert-core folds incidents by; NULL on an error row.
    fingerprint      TEXT,
    tier             TEXT        NOT NULL CHECK (tier IN ('priority', 'standard')),
    -- The queue's name and message id (such as sqs/priority-queue/<id>) and the alert's position in it; a redelivered message stores nothing new.
    message_id       TEXT        NOT NULL,
    message_index    INTEGER     NOT NULL CHECK (message_index >= 0),
    -- The request body exactly as the source sent it, kept on the message's first row (message_index 0) only.
    raw_body         BYTEA,
    -- The Content-Type the source sent the body with, when the gateway passed one on; first row only, like raw_body.
    content_type     TEXT,
    -- Why the message could not be normalised (MissingSource, UnknownSource, TransformFailed, TooManyAlerts, UnstorableAlert) and the detail; NULL on an alert row.
    error_reason     TEXT,
    error            TEXT,
    -- When the gateway accepted the request, which dedup windows are measured on.
    received_on      TIMESTAMPTZ NOT NULL,
    created_on       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Claim columns for the alert processor, which takes unprocessed rows with FOR UPDATE SKIP LOCKED.
    claimed_by       TEXT,
    claim_expires_on TIMESTAMPTZ,
    processed_on     TIMESTAMPTZ,
    -- The incident this alert belongs to; NULL until the alert processor links it, and again if the incident is deleted.
    incident_id      UUID        REFERENCES incident(id) ON DELETE SET NULL,
    CONSTRAINT alert_message_key UNIQUE (message_id, message_index),
    -- The body travels on the first row of each message and on no other.
    CONSTRAINT alert_raw_body_first_row CHECK ((message_index = 0) = (raw_body IS NOT NULL)),
    -- An alert row has a source, alert and fingerprint and no error; an error row is the message's only row, with an error and no alert.
    CONSTRAINT alert_row_kind CHECK (
        (error_reason IS NULL AND error IS NULL AND source IS NOT NULL AND alert IS NOT NULL AND fingerprint IS NOT NULL)
        OR (error_reason IS NOT NULL AND btrim(error_reason) <> '' AND alert IS NULL AND fingerprint IS NULL AND message_index = 0 AND incident_id IS NULL)
    )
) WITH (fillfactor = 85, autovacuum_vacuum_scale_factor = 0.05, autovacuum_vacuum_insert_scale_factor = 0.05);

-- The alert processor seeds a claim from the oldest unprocessed alert ids; error rows never become incidents.
CREATE INDEX IF NOT EXISTS alert_unprocessed_idx ON alert (id) WHERE processed_on IS NULL AND error_reason IS NULL;
-- The alert processor pulls the rest of a fingerprint into the same claim, so one incident's alerts fold together.
CREATE INDEX IF NOT EXISTS alert_unprocessed_fingerprint_idx ON alert (fingerprint) WHERE processed_on IS NULL AND error_reason IS NULL;
-- Finds every alert of one incident, and keeps a deleted incident's SET NULL from scanning the table.
CREATE INDEX IF NOT EXISTS alert_incident_id_idx ON alert (incident_id) WHERE incident_id IS NOT NULL;
-- Lists the messages that could not be normalised, newest first.
CREATE INDEX IF NOT EXISTS alert_error_idx ON alert (received_on DESC) WHERE error_reason IS NOT NULL;

COMMENT ON TABLE alert IS
    'SRE alert messages the alert consumer stored: one row per normalised alert, or one error row for a message that could not be normalised, deduplicated by queue message.';
COMMENT ON COLUMN alert.incident_id IS
    'The incident this alert belongs to; NULL until the alert processor links it.';
