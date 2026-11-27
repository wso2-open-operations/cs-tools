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

-- SRE alerts v2: the alert table. The queue consumer stores every alert it receives here, one row per alert in a
-- message (or one error row for a message it could not read), and acknowledges the queue message only after the rows
-- commit. The row is also the outbox of alert.created (published_on) and carries the alert processor's outcome.
-- Safe to run again.
SET lock_timeout = '5s';

CREATE SEQUENCE IF NOT EXISTS alert_seq AS BIGINT START 1;

CREATE TABLE IF NOT EXISTS alert (
    id                 TEXT        PRIMARY KEY,
    -- The source route the gateway named (prometheus, azure, aws, ...); NULL only on an error row that named none.
    source             TEXT,
    -- The canonical alert (alert-core's model) and its fingerprint; NULL on an error row.
    alert              JSONB       CHECK (jsonb_typeof(alert) = 'object'),
    fingerprint        TEXT,
    -- 'high' (a priority queue) or 'default' (a standard queue).
    tier               TEXT        NOT NULL CHECK (tier IN ('high', 'default')),
    -- The queue message this row came from; one message can carry several alerts.
    message_id         TEXT        NOT NULL,
    message_index      INTEGER     NOT NULL CHECK (message_index >= 0),
    -- The message body exactly as the source sent it, kept on the message's first row only.
    raw_body           BYTEA,
    content_type       TEXT,
    -- Why the message could not be turned into alerts; set on an error row only.
    error_reason       TEXT,
    error              TEXT,
    received_on        TIMESTAMPTZ NOT NULL,
    created_on         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- NEW waits for the processor; ERROR is a message that could not be read and is never processed; the rest are the
    -- processor's outcomes.
    status             TEXT        NOT NULL DEFAULT 'NEW',
    -- The relay took the alert to publish alert.created; cleared again when the publish or the decision fails.
    published_on       TIMESTAMPTZ,
    -- The processor's lease on the alert while it decides.
    claimed_by         TEXT,
    claim_expires_on   TIMESTAMPTZ,
    attempts           INTEGER     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error         TEXT,
    next_attempt_on    TIMESTAMPTZ,
    processed_on       TIMESTAMPTZ,
    incident_id        UUID        REFERENCES incident (id) ON DELETE SET NULL,
    shadow_decision    TEXT,
    shadow_incident_id UUID,
    CONSTRAINT alert_message_key UNIQUE (message_id, message_index),
    CONSTRAINT alert_raw_body_first_row CHECK ((message_index = 0) = (raw_body IS NOT NULL)),
    CONSTRAINT alert_status_check CHECK (status IN ('NEW', 'ERROR', 'PROCESSED', 'SHADOWED', 'FAILED')),
    CONSTRAINT alert_error_row_check CHECK ((status = 'ERROR') = (error_reason IS NOT NULL)),
    CONSTRAINT alert_readable_row_check CHECK (error_reason IS NOT NULL OR (alert IS NOT NULL AND fingerprint IS NOT NULL)),
    CONSTRAINT alert_shadow_decision_check CHECK (shadow_decision IN ('WOULD_CREATE', 'WOULD_ATTACH', 'WOULD_IGNORE'))
);

-- What the relay publishes next: waiting alerts not yet published, priority tier first, then oldest.
CREATE INDEX IF NOT EXISTS alert_unpublished_idx ON alert (tier DESC, id)
    WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NULL;
-- Published and still undecided: the relay's backpressure count and the stale sweep.
CREATE INDEX IF NOT EXISTS alert_in_flight_idx ON alert (published_on)
    WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NOT NULL;
-- An incident's alerts.
CREATE INDEX IF NOT EXISTS alert_incident_id_idx ON alert (incident_id) WHERE incident_id IS NOT NULL;
-- The newest messages that could not be read, for whoever looks into a source.
CREATE INDEX IF NOT EXISTS alert_error_idx ON alert (received_on DESC) WHERE error_reason IS NOT NULL;

-- The rows carry their own outcome and are written only by the consumer, the relay and the processor, so the alert
-- table is left out of the field-level audit log (where that log is installed).
DO $$
BEGIN
    IF to_regprocedure('exclude_from_audit(text, text, text)') IS NOT NULL THEN
        PERFORM exclude_from_audit(current_schema(), 'alert',
            'SRE alert queue table: written by the alert consumer, relay and processor only; rows carry their own outcome');
    END IF;
END $$;

COMMENT ON TABLE alert IS 'SRE alerts v2: every alert the queue consumer stored, the outbox of alert.created, and the alert processor''s outcome.';
COMMENT ON COLUMN alert.status IS
    'NEW: waiting for the processor. ERROR: the message could not be read (error_reason, error); never processed. '
    'PROCESSED: linked to its incident (incident_id), or a resolving alert with no open incident. '
    'SHADOWED: hybrid mode recorded shadow_decision and raised nothing. FAILED: gave up after repeated errors (last_error).';
COMMENT ON COLUMN alert.published_on IS
    'When the alert relay took this waiting alert to publish alert.created. NULL with processed_on NULL: still to publish '
    '(new, or a failed attempt waiting for next_attempt_on).';
