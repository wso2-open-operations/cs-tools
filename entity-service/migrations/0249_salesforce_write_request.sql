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

-- salesforce_write_request records a portal membership write whose Salesforce
-- half could not run because sales/sales-entity-service (or Salesforce behind
-- it) was unavailable. The CSM rows are committed in the same transaction as
-- this row, with their sf_id columns NULL. The write itself is then carried
-- out by sales-entity-service: entity-service publishes the command
-- (salesforce.membership_write.requested, topic SALESFORCE_WRITE_EVENT_HUB_TOPIC),
-- Sales Entity performs the Salesforce write and answers on
-- SALESFORCE_WRITE_RESULTS_TOPIC, and entity-service's results consumer
-- (internal/service/project_membership_deferred_write.go) marks the row
-- SUCCEEDED in the transaction that stamps the Salesforce ids, or FAILED.
--
-- WHY A ROW AND NOT ONLY AN EVENT: the event is the trigger, this row is the
-- record. Event Hub commits the offset after a bounded number of attempts
-- and the dead-letter tier drops what it cannot deliver, so without a row
-- nothing would say a membership is still owed to Salesforce. The portals
-- read status from here ("syncing to Salesforce"), and a later retry sweep
-- or a manual retry re-publishes from here.
--
-- intent is the portal request as the admin made it (address, names, roles,
-- state, integration-user flag), replayed verbatim by the consumer so the
-- Salesforce record ends up exactly as a synchronous write would have made
-- it. Only the INVITE operation is deferred; role changes and resend need
-- the Salesforce ids and are refused (409) while a row here is PENDING or
-- FAILED. Deactivation of a FAILED (or never published) row runs locally and
-- moves the row to CANCELLED: there is no Salesforce half to undo.
--
-- retryable tells a FAILED row's two causes apart: TRUE when the retries ran
-- out while Salesforce was unavailable (a re-publish would likely succeed),
-- FALSE when Salesforce refused the write (the same request would be refused
-- again). A sweep or an operator retries only the former.
BEGIN;

CREATE TABLE IF NOT EXISTS salesforce_write_request (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_on         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    operation          VARCHAR(20)  NOT NULL,
    project_id         UUID         NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    project_contact_id UUID         REFERENCES project_contact(id) ON DELETE CASCADE,
    email              VARCHAR(255) NOT NULL,
    intent             JSONB        NOT NULL,
    status             VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    -- Every Salesforce attempt the consumer made, whatever tier it ran on.
    attempts           INTEGER      NOT NULL DEFAULT 0,
    -- Bounded and never the raw upstream body (see github_outbound_queue).
    last_error         VARCHAR(500),
    last_attempt_on    TIMESTAMPTZ,
    -- When the salesforce.membership_write.requested event was acknowledged
    -- by Event Hub. NULL means the publish failed and nothing will consume
    -- this row until something re-publishes it.
    published_on       TIMESTAMPTZ,
    completed_on       TIMESTAMPTZ,
    retryable          BOOLEAN      NOT NULL DEFAULT FALSE,
    CONSTRAINT chk_salesforce_write_request_operation CHECK (operation IN ('INVITE')),
    CONSTRAINT chk_salesforce_write_request_status    CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED', 'CANCELLED'))
);

-- One open request per membership row: a second deferral of the same row
-- while the first is still owed is a bug, not a second write.
CREATE UNIQUE INDEX IF NOT EXISTS uq_salesforce_write_request_pending
    ON salesforce_write_request (project_contact_id)
    WHERE status = 'PENDING';

-- The sweep / operator query: what is still owed, oldest first.
CREATE INDEX IF NOT EXISTS idx_salesforce_write_request_open
    ON salesforce_write_request (created_on)
    WHERE status IN ('PENDING', 'FAILED');

COMMIT;
