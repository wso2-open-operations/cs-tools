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

-- cmdb_outage_ci_mtom: which configuration items an outage affects.
--
-- ServiceNow's `Cloud Status Event Notification Flow` reads this to decide
-- whose cloud-status entry to rewrite -- for each affected CI it finds that
-- CI's monitor and sets a status. Without the table mirrored, the Go port has
-- no way to answer "what else does this outage take down", so it is a hard
-- prerequisite rather than an enrichment.
--
-- A pure join table: 132 rows on the source instance, three columns, no
-- payload of its own.

CREATE TABLE IF NOT EXISTS outage_affected_ci (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,

    outage_id UUID REFERENCES outage(id) ON DELETE CASCADE,

    -- Deliberately NOT a foreign key, and deliberately nullable.
    --
    -- The source column references cmdb_ci, the base class every CI type
    -- extends -- so a row may point at a service, a service offering, an
    -- application or anything else. Postgres mirrors those into separate
    -- tables, so there is no single table to reference. The port resolves it
    -- against service_offering (which is what the cloud-status flow needs)
    -- and skips what it cannot place.
    --
    -- Nullable because it genuinely is: one of the 132 source rows has no CI
    -- at all. Making it NOT NULL would fail that row's sync forever and lose
    -- the outage's other affected CIs with it.
    ci_id UUID,

    sync_time_stamp TIMESTAMPTZ
);

-- The port's only query: every affected CI for one outage.
CREATE INDEX IF NOT EXISTS idx_outage_affected_ci_outage
    ON outage_affected_ci (outage_id);

-- And the reverse, for "which outages currently affect this CI".
CREATE INDEX IF NOT EXISTS idx_outage_affected_ci_ci
    ON outage_affected_ci (ci_id)
    WHERE ci_id IS NOT NULL;
