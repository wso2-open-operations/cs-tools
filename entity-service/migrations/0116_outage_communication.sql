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

-- Everything ServiceNow's `Outage Communication` flow reads or writes that
-- this mirror does not yet carry. Its Go port is cs-tools #2200, which is
-- inert until all of this lands.

-- ---------------------------------------------------------------------------
-- 1. THE GATE, AND TWO FIELDS THE EMAIL RENDERS.
-- ---------------------------------------------------------------------------

-- u_outage_communication gates the flow entirely: its first Wait For
-- Condition is `beginISNOTEMPTY^u_outage_communication=true`. Without this
-- column the port's sweep finds no candidates at all, ever.
--
-- It is an opt-in a human sets, never written back by the flow -- its own
-- pill metadata records it as used only in conditions.
ALTER TABLE outage ADD COLUMN IF NOT EXISTS outage_communication BOOLEAN;

-- impact and state are printed in both emails ("Impact:" and
-- "Current Status:") and neither is mirrored today, under these names or any
-- others -- checked against the live dev schema rather than assumed. Without
-- them both lines render blank.
--
-- Plain text, not enums: these are task-derived fields whose ServiceNow
-- choice lists are not stable enough to pin, and a CREATE TYPE that rejects a
-- real value fails the whole row.
ALTER TABLE outage ADD COLUMN IF NOT EXISTS impact VARCHAR(40);
ALTER TABLE outage ADD COLUMN IF NOT EXISTS state  VARCHAR(40);

-- ---------------------------------------------------------------------------
-- 2. THE COMMUNICATION LOG.
-- ---------------------------------------------------------------------------

-- One row per email the outage-communication flows have sent. 336 rows on the
-- dev instance.
--
-- *** NOT `outage_communication`. *** That table is the public JOURNAL -- the
-- external updates the status page renders verbatim (cs-tools migration
-- 000105). This one is an internal send-log: full email bodies, recipient
-- lists, subjects. The names are one suffix apart and the contents sit on
-- opposite sides of a disclosure boundary.
--
-- *** AND IT IS THE PORT'S IDEMPOTENCY GUARD, WHICH IS WHY THE BACKFILL IS
-- NOT OPTIONAL. *** ServiceNow's flow is "Run Trigger: Once" -- the platform
-- starts one instance per outage and that instance sends each email exactly
-- once, so the flow writes NO state to the outage at all. A scheduled sweep
-- inherits none of that, and reconstructs the guarantee by asking "is there
-- already a DECLARED row for this outage?".
--
-- Sync this table empty and 95 already-announced outages are announced again.
CREATE TABLE IF NOT EXISTS outage_communication_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- *** JOIN ON THE NUMBER, NOT A FOREIGN KEY. *** ServiceNow's table has a
    -- real u_outage reference column and it is empty on all 336 rows; the
    -- link has always been the number string. Mirroring the reference would
    -- produce a column that is null forever.
    outage_number   VARCHAR(32) NOT NULL,

    -- *** FREE TEXT, DELIBERATELY NOT AN ENUM. *** The live data holds six
    -- distinct values -- Update 155, Declared 88, Resolve 72, RCA 13,
    -- Declare 7, and one blank. Two are spellings of each other and 13 come
    -- from a flow that is inactive today. A CREATE TYPE would reject real
    -- rows at import.
    email_type      TEXT,

    -- Normalised for querying, so the guard cannot be defeated by the
    -- Declare/Declared split. Generated rather than written, so it can never
    -- drift from email_type.
    --
    -- Matching on the PREFIX is the point: `= 'DECLARED'` misses the 7 rows
    -- spelled "Declare" and re-announces those outages at cutover.
    email_type_norm TEXT GENERATED ALWAYS AS (
        CASE
            WHEN email_type IS NULL THEN NULL
            WHEN upper(email_type) LIKE 'DECLARE%' THEN 'DECLARED'
            WHEN upper(email_type) LIKE 'RESOLV%'  THEN 'RESOLVED'
            WHEN upper(email_type) LIKE 'UPDATE%'  THEN 'UPDATE'
            WHEN upper(email_type) LIKE 'RCA%'     THEN 'RCA'
            ELSE upper(email_type)
        END
    ) STORED,

    outage_status   VARCHAR(40),
    subject         VARCHAR(100),
    recipients      VARCHAR(4000),
    email_content   TEXT,
    main_content    VARCHAR(1000),
    sent_on         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    created_on      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      VARCHAR(255) NOT NULL DEFAULT 'system',
    updated_by      VARCHAR(255) NOT NULL DEFAULT 'system'
);

-- The sweep's only question: what have we already said about this outage?
CREATE INDEX IF NOT EXISTS idx_outage_communication_log_guard
    ON outage_communication_log (outage_number, email_type_norm);

-- Operational reads are newest-first.
CREATE INDEX IF NOT EXISTS idx_outage_communication_log_sent
    ON outage_communication_log (sent_on DESC);
