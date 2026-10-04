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

-- What this service has already told the cloud status dashboard.
--
-- The port of ServiceNow's `Cloud Status Event Notification Flow`, steps 7/8
-- and 14/15: when an in-scope outage begins or ends, POST a webhook to the
-- status dashboard. The flow's other half -- rewriting cloud_monitor.status
-- for every affected CI -- belongs to a separate service and is deliberately
-- not reproduced here.
--
-- A record of SENDS, not of outage state. The outage itself lives in the
-- mirrored `outage` table, which csm-sync-service owns and this service must
-- never write; the same reasoning that keeps outage_notifications separate.
-- What is ours is the answer to "have we already told the dashboard about
-- this transition", and nothing else can answer it: the dashboard has no
-- read-back and the webhook is fire-and-forget.

DO $$ BEGIN
    -- The `event` string the webhook body carries. ServiceNow sends these as
    -- bare literals typed into the action's input.
    --
    -- OUTAGE_BEGIN is confirmed from the flow (step 15). OUTAGE_END is
    -- INFERRED for step 8 and has NOT been seen -- see the port's own
    -- constant for why that matters and what to check.
    CREATE TYPE cloud_status_event_enum AS ENUM (
        'OUTAGE_BEGIN', 'OUTAGE_END'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS cloud_status_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Not a foreign key: while csm-sync-service still owns `outage`, a
    -- migration there could recreate the table, cascading away our send
    -- history and re-posting every transition. Same choice sla_clocks and
    -- outage_notifications make.
    --
    -- That risk ends at cutover, when the sync is retired and `outage`
    -- becomes natively owned -- at which point this could become a real
    -- foreign key. Left as a plain column deliberately rather than
    -- revisited then: re-posting every historical transition to a public
    -- status page is a bad enough outcome that the constraint is not worth
    -- the tidiness.
    outage_id UUID NOT NULL,

    event cloud_status_event_enum NOT NULL,

    -- The cloud offering the webhook was routed by, stored as sent rather
    -- than re-derived. An outage's offering can be corrected after the fact,
    -- and "what did we actually post" must not change when it is.
    cloud VARCHAR(40) NOT NULL,

    -- Delivery outcome. A webhook that failed is worth keeping rather than
    -- discarding: it is the only evidence the dashboard is behind, and the
    -- retry decision needs to see it.
    delivered      BOOLEAN NOT NULL DEFAULT FALSE,
    attempt_count  INTEGER NOT NULL DEFAULT 0,
    last_error     TEXT,
    last_attempt_on TIMESTAMPTZ,
    delivered_on   TIMESTAMPTZ,

    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One send per outage per transition PER CLOUD.
    --
    -- The idempotency guarantee: ServiceNow re-fires on every qualifying
    -- record update and relies on nothing, so a port that swept without this
    -- would re-post the same begin event on every pass.
    --
    -- *** CLOUD IS PART OF THE KEY, AND THAT IS NOT AN OPTIMISATION. *** One
    -- outage can touch more than one cloud: the flow being ported has a
    -- sibling, `Cloud Status Event Notification Flow - Affected CI`, that
    -- triggers on the affected-CI join table and routes its webhook by THAT
    -- CI's cloud rather than the outage's own. So a Devant outage that also
    -- affects an Asgardeo component tells both dashboards, and each needs its
    -- own refresh -- they are separate deployments with separate data.
    --
    -- Keying on (outage_id, event) alone silently suppressed the second
    -- cloud as a duplicate, which would have left one public status page
    -- stale with nothing logged.
    UNIQUE (outage_id, event, cloud)
);

-- The sweep's read: transitions still owed a successful delivery, oldest
-- first.
CREATE INDEX IF NOT EXISTS idx_cloud_status_events_undelivered
    ON cloud_status_events (created_on)
    WHERE delivered IS FALSE;
