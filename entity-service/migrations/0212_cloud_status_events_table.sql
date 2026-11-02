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

-- cloud_status_events, created where the 4-digit sequence can see it.
--
-- The table and its enum are defined by the legacy straggler
-- 000086_create_cloud_status_events, which every runner (make migrate,
-- generate_schema_bootstrap.sh, the compose stack's migrate-and-seed.sh)
-- applies after every 4-digit file. 0213_cloud_status_event_delivery_lease
-- alters this table, so on a fresh database it would run before the table
-- existed and stop the whole run:
-- `relation "cloud_status_events" does not exist`.
--
-- Numbered 0212 rather than 0213 so it sorts ahead of 0213 under any
-- collation, not by a name comparison that a locale-aware sort could
-- reorder. It depends on nothing else.
--
-- Same definition as 000086, the enum, table and index each guarded: a no-op
-- wherever 000086 already ran, and 000086 is a no-op after this one. 000086
-- stays as it is, since databases have it recorded by filename. See 000086
-- for the notes on the columns, the missing foreign key and the unique key.
--
-- The plural name is 000086's and 0213 alters it under that name, so it is
-- kept: see the table-naming note in entity-service/CLAUDE.md.
--
-- Where the table already exists, CREATE INDEX IF NOT EXISTS still takes a
-- SHARE lock on it before it finds the index. That lock waits behind open
-- writes and holds up the writes queued after it, so lock_timeout makes a
-- busy table fail the file instead; run it again.
--
-- No BEGIN/COMMIT: the three CREATEs are guarded, so a run that stops
-- partway is finished by the next one, and migrate-and-seed.sh applies this
-- file with `psql -1` in one transaction with its tracking insert, which a
-- COMMIT here would split.
SET lock_timeout = '5s';

DO $$ BEGIN
    -- The `event` string the webhook body carries.
    CREATE TYPE cloud_status_event_enum AS ENUM (
        'OUTAGE_BEGIN', 'OUTAGE_END'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS cloud_status_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Not a foreign key, on purpose: see 000086.
    outage_id UUID NOT NULL,

    event cloud_status_event_enum NOT NULL,

    -- The cloud offering the webhook was routed by, stored as sent.
    cloud VARCHAR(40) NOT NULL,

    delivered      BOOLEAN NOT NULL DEFAULT FALSE,
    attempt_count  INTEGER NOT NULL DEFAULT 0,
    last_error     TEXT,
    last_attempt_on TIMESTAMPTZ,
    delivered_on   TIMESTAMPTZ,

    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One send per outage per transition per cloud: see 000086.
    UNIQUE (outage_id, event, cloud)
);

-- The sweep's read: transitions still owed a successful delivery, oldest
-- first.
CREATE INDEX IF NOT EXISTS idx_cloud_status_events_undelivered
    ON cloud_status_events (created_on)
    WHERE delivered IS FALSE;

RESET lock_timeout;
