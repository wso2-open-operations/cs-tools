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

-- The customer-facing updates the public status page prints under an
-- incident.
--
-- WHY A TABLE AND NOT THE EXISTING COLUMN. `outage` already carries
-- external_outage_communications as a single TEXT column, and it is
-- populated on ZERO of 637 rows. In ServiceNow the same field is a JOURNAL:
-- sys_journal_field holds one row per update, each with its own timestamp,
-- and the detail endpoint publishes them as a list ordered newest first. A
-- single column can hold the latest text but not the history, and not the
-- timestamps the list is built from. The live API returns multi-entry
-- comment lists today, so this is a gap with real data behind it.
--
-- WHY IT LIVES HERE AND NOT IN A SYNC MAPPING. There is no ongoing sync:
-- csm-sync-service is a one-time bulk migration, and after cutover
-- ServiceNow is gone and outages are written directly to Postgres. This is
-- therefore a first-class table this service owns, not a mirror of one.
--
-- The existing outage.external_outage_communications column is deliberately
-- left in place. Dropping a column something else may still write is a
-- separate decision from adding the one this endpoint reads.
BEGIN;

CREATE TABLE IF NOT EXISTS outage_communication (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outage_id UUID NOT NULL REFERENCES outage(id) ON DELETE CASCADE,
    -- Verbatim, including ServiceNow's [code]...[/code] wrappers around
    -- HTML. The frontend renders what it is given, so stripping them here
    -- would change how every existing update displays.
    comment TEXT NOT NULL,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255)
);

-- The endpoint's only access path: one outage, newest first.
CREATE INDEX IF NOT EXISTS idx_outage_communication_outage
    ON outage_communication (outage_id, created_on DESC);

COMMIT;
