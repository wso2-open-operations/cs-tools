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

-- outage_communication, created where the 4-digit sequence can see it.
--
-- The table is defined by the legacy straggler 000105_outage_communications,
-- and every runner (make migrate, generate_schema_bootstrap.sh, the compose
-- stack's migrate-and-seed.sh) applies those stragglers after every 4-digit
-- file. 0184_outage_communication_channel alters this table, so on a fresh
-- database it ran before the table existed and stopped the whole run:
-- `relation "outage_communication" does not exist`.
--
-- Numbered 0183 rather than 0184 so it sorts ahead of both 0184 files under
-- any collation, not by a name comparison that a locale-aware sort could
-- reorder. It needs only `outage` (0083).
--
-- Same definition as 000105, table and index both IF NOT EXISTS: a no-op
-- wherever 000105 already ran, and 000105 is a no-op after this one. 000105
-- stays as it is, since databases have it recorded by filename. See 000105
-- for why this is a table rather than outage.external_outage_communications.
--
-- Where the table already exists, CREATE INDEX IF NOT EXISTS still takes a
-- SHARE lock on it before it finds the index. Where it does not, the foreign
-- key takes a SHARE ROW EXCLUSIVE lock on outage. Either lock waits behind
-- open writes and holds up the writes queued after it, so lock_timeout makes
-- a busy table fail the file instead; run it again.
--
-- No BEGIN/COMMIT: both CREATEs are guarded, so a run that stops partway is
-- finished by the next one, and migrate-and-seed.sh applies this file with
-- `psql -1` in one transaction with its tracking insert, which a COMMIT here
-- would split.
SET lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS outage_communication (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outage_id UUID NOT NULL REFERENCES outage(id) ON DELETE CASCADE,
    -- Verbatim, including the [code]...[/code] wrappers the backing data
    -- source puts around HTML. The frontend renders what it is given, so
    -- stripping them here would change how existing updates display.
    comment TEXT NOT NULL,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255)
);

-- Every read is for one outage, and the lists read its rows newest first.
CREATE INDEX IF NOT EXISTS idx_outage_communication_outage
    ON outage_communication (outage_id, created_on DESC);

RESET lock_timeout;
