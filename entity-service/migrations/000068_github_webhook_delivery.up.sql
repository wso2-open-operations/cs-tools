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

-- Every webhook delivery we have accepted, so we never act on one twice.
--
-- WHY THIS EXISTS AND THE NOTIFICATION FLOWS DID NOT NEED IT. Those published
-- email; a duplicate was an annoyance. These mutate -- a replayed delivery
-- creates a second change request or posts a second comment. GitHub retries on
-- any non-2xx and on operator request, and it REUSES X-GitHub-Delivery when it
-- does, which makes that header a real idempotency key rather than a guess.
--
-- The row is inserted BEFORE the work, inside the same transaction. A conflict
-- means another attempt already has it, so this one stops. Rolling back on
-- failure lets the retry through, which is what a retry is for.
CREATE TABLE IF NOT EXISTS github_webhook_delivery (
    -- GitHub's delivery GUID, verbatim.
    delivery_id VARCHAR(100) PRIMARY KEY,
    received_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Kept for diagnosis: which event and action this delivery carried.
    event VARCHAR(50) NOT NULL,
    action VARCHAR(50),
    -- The change request it resolved to, when it resolved to one. Null for a
    -- delivery we accepted and then found nothing to do with.
    change_request_id UUID REFERENCES change_request(id) ON DELETE SET NULL
);

-- Deliveries are only ever read by primary key. This index exists for the
-- retention sweep, which deletes by age: without it that becomes a full scan
-- on a table that only grows.
CREATE INDEX IF NOT EXISTS idx_github_webhook_delivery_received
    ON github_webhook_delivery (received_on);
