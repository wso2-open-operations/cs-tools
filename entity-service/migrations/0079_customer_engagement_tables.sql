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

-- Customer engagements, resource allocations and weekly status updates,
-- mirrored from ServiceNow.
--
-- Three custom u_ tables form one cluster around an ENGAGEMENT -- a piece of
-- customer work that people are allocated to for a date range, and that those
-- people are expected to write a short status update against every week:
--
--   engagement                      the engagement; state gates activity
--   customer_engagement_allocation_resource  who is allocated, and for how long
--   customer_engagement_status_update        one weekly update, by one author
--
-- WHY THIS EXISTS NOW: cs-tools is porting ServiceNow's
-- WeeklyAllocationStatusUpdateReminderEmailFlow to
-- operations/csm-scheduled-tasks. That flow reads all three tables to work out
-- who owes an update for last week, and none of them had a Postgres home.
--
-- Column lists are field-for-field against the sys_dictionary dump taken
-- 2026-09-21 (cs-tools docs/servicenow-discovery/41-engagement-allocation-tables.js
-- PASS 1); every choice list below is PASS 2's, verbatim.
--
-- ---------------------------------------------------------------------------
-- THREE THINGS THE DICTIONARY REVEALED THAT THE FLOW IGNORES
-- ---------------------------------------------------------------------------
-- All three are mapped here precisely so the port can decide, rather than
-- inheriting ServiceNow's silence:
--
--   1. ENGAGEMENT STATE. The flow filters on the raw value "1" only, which is
--      IN_PROGRESS. NEW, REQUESTED and ON_HOLD engagements are therefore never
--      reminded about, whether or not people are actively allocated to them.
--
--   2. ALLOCATION STATE. This table has its own state -- including
--      ALLOCATION_CANCELLED, REJECTED_BY_CONSULTANT and REJECTED_OTHER -- and
--      the flow does not look at it at all. A cancelled allocation produces a
--      reminder today.
--
--   3. STATUS UPDATE STATE. Updates can be DRAFT, PUBLISHED or CANCELLED, and
--      the flow counts any row regardless. Starting a draft and abandoning it
--      suppresses the reminder exactly as well as publishing one.
--
-- ---------------------------------------------------------------------------
-- REFERENCES THAT ARE DELIBERATELY NOT FOREIGN KEYS
-- ---------------------------------------------------------------------------
-- A reference only becomes a FK when its target table is itself synced,
-- otherwise the loader would drop every row whose parent has not been seen.
-- Synced, so real FKs: customer_account, customer_project, "user" (sys_user).
-- NOT synced, so plain sys_id text: u_customer_engagement_type,
-- u_sf_opportunity, u_sf_opportunity_product.
--
-- u_parent (engagement -> engagement) is left out of this initial version
-- entirely, for the same reason "group".parent_id was in migration 0074: the
-- full migration pages in sys_id order, not hierarchy order, so a child can be
-- inserted before its parent and violate the FK. Add it as a field_backfill
-- once every engagement row exists.

-- ---------- enums ----------
-- Keys in each mapping YAML's value_map are the RAW ServiceNow choice values,
-- which for u_state are numeric strings and elsewhere are the labels
-- themselves. See the YAMLs for the exact mapping.

DO $$ BEGIN
    CREATE TYPE customer_engagement_state_enum AS ENUM
        ('NEW', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED', 'ON_HOLD', 'REQUESTED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE customer_engagement_business_unit_enum AS ENUM
        ('API_AND_INTEGRATION_SOFTWARE', 'CHOREO', 'IAM');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE customer_engagement_consumption_mode_enum AS ENUM
        ('ONE_TIME_FIXED_START', 'ONE_TIME_FLEXIBLE_START', 'MULTIPLE_SPANS');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE customer_engagement_delivery_mode_enum AS ENUM ('OFFSITE', 'ONSITE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Nine values, and the three rejection/cancellation ones are the reason this
-- column is worth mapping at all -- see note 2 in the header.
DO $$ BEGIN
    CREATE TYPE engagement_allocation_state_enum AS ENUM (
        'TENTATIVE',
        'READY_FOR_CLEARANCE',
        'CONFIRMED',
        'CLEARANCE_IN_PROGRESS',
        'ALLOCATION_CANCELLED',
        'ACCEPTED_BY_CONSULTANT',
        'REJECTED_OTHER',
        'REJECTED_BY_CONSULTANT',
        'CONFIRMED_VISA_PENDING'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE customer_engagement_status_update_state_enum AS ENUM ('DRAFT', 'PUBLISHED', 'CANCELLED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE customer_engagement_status_update_frequency_enum AS ENUM
        ('DAILY', 'WEEKLY', 'BIWEEKLY', 'MONTHLY', 'ON_DEMAND');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE customer_engagement_status_update_scope_enum AS ENUM ('INDIVIDUAL', 'PROJECT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Raw values are "0"/"1"; the labels are Internal/Customer.
DO $$ BEGIN
    CREATE TYPE customer_engagement_status_update_visibility_enum AS ENUM ('INTERNAL', 'CUSTOMER');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------- the engagement ----------
CREATE TABLE IF NOT EXISTS customer_engagement (
    id                  UUID PRIMARY KEY,
    created_on          TIMESTAMPTZ NOT NULL,
    updated_on          TIMESTAMPTZ NOT NULL,
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    name                VARCHAR(200),
    engagement_id       VARCHAR(20),
    engagement_code     VARCHAR(20),
    state               customer_engagement_state_enum,
    business_unit       customer_engagement_business_unit_enum,
    consumption_mode    customer_engagement_consumption_mode_enum,
    delivery_mode       customer_engagement_delivery_mode_enum,
    is_paid             BOOLEAN,
    account_id          UUID REFERENCES account(id) ON DELETE SET NULL,
    project_id          UUID REFERENCES project(id) ON DELETE SET NULL,
    lead_id             UUID REFERENCES "user"(id) ON DELETE SET NULL,
    owner_id            UUID REFERENCES "user"(id) ON DELETE SET NULL,
    -- Unsynced reference targets -- raw sys_ids, not FKs. See the header.
    engagement_type_id  VARCHAR(32),
    opportunity_id      VARCHAR(32),
    line_item_id_ref    VARCHAR(32),
    -- u_line_item_id is a separate string(40) field from the u_line_item
    -- reference above, despite the near-identical name.
    line_item_id        VARCHAR(40),
    planned_start_date  DATE,
    planned_end_date    DATE,
    actual_start_date   DATE,
    actual_end_date     DATE
);

-- The reminder query narrows to active engagements first, and that is the only
-- predicate this table contributes.
CREATE INDEX IF NOT EXISTS idx_customer_engagement_state   ON customer_engagement (state);
CREATE INDEX IF NOT EXISTS idx_customer_engagement_account ON customer_engagement (account_id);
CREATE INDEX IF NOT EXISTS idx_customer_engagement_project ON customer_engagement (project_id);

-- ---------- who is allocated ----------
CREATE TABLE IF NOT EXISTS customer_engagement_allocation_resource (
    id            UUID PRIMARY KEY,
    created_on    TIMESTAMPTZ NOT NULL,
    updated_on    TIMESTAMPTZ NOT NULL,
    created_by    VARCHAR(255),
    updated_by    VARCHAR(255),
    -- u_allocation_id: a human-readable reference, not a sys_id.
    allocation_id VARCHAR(20),
    -- Nullable despite being dictionary-mandatory: PASS 3 found 3 existing
    -- rows with it empty. The mapping's source_filter drops those rather than
    -- loading rows the reminder query could never join anyway.
    engagement_id UUID REFERENCES customer_engagement(id) ON DELETE CASCADE,
    resource_id   UUID REFERENCES "user"(id) ON DELETE SET NULL,
    state         engagement_allocation_state_enum,
    -- DATE, not TIMESTAMPTZ: the source fields are glide_date (no time part),
    -- and the flow compares them against a GlideDate. Storing them as
    -- timestamptz would invent a midnight instant in some timezone and shift
    -- the boundary for anyone reading them from a different one.
    start_date    DATE,
    end_date      DATE,
    -- The time-of-day halves are string(8) upstream ("09:00:00"), kept as text
    -- rather than TIME: they are wall-clock in timezone below, not instants,
    -- and nothing queries on them.
    start_time    VARCHAR(8),
    end_time      VARCHAR(8),
    timezone      VARCHAR(20),
    -- percent_complete upstream; a percentage, so NUMERIC not INTEGER.
    utilization   NUMERIC(6,2)
);

CREATE INDEX IF NOT EXISTS idx_cea_resource_engagement ON customer_engagement_allocation_resource (engagement_id);
CREATE INDEX IF NOT EXISTS idx_cea_resource_resource   ON customer_engagement_allocation_resource (resource_id);
CREATE INDEX IF NOT EXISTS idx_cea_resource_dates      ON customer_engagement_allocation_resource (start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_cea_resource_state      ON customer_engagement_allocation_resource (state);

-- ---------- the weekly update ----------
CREATE TABLE IF NOT EXISTS customer_engagement_status_update (
    id               UUID PRIMARY KEY,
    created_on       TIMESTAMPTZ NOT NULL,
    updated_on       TIMESTAMPTZ NOT NULL,
    created_by       VARCHAR(255),
    updated_by       VARCHAR(255),
    engagement_id    UUID REFERENCES customer_engagement(id) ON DELETE CASCADE,
    -- A real FK: PASS 3 found 0 rows with an empty reference on this table.
    allocation_id    UUID REFERENCES customer_engagement_allocation_resource(id) ON DELETE SET NULL,
    -- Who wrote it. The reminder is per-person, so this column is what makes
    -- "has THIS resource updated?" answerable at all; without it the check
    -- degrades to "has ANYONE on the engagement updated?", which is exactly
    -- the bug the ServiceNow original shipped with.
    author_id        UUID REFERENCES "user"(id) ON DELETE SET NULL,
    subject          VARCHAR(160),
    -- html(8000) upstream. TEXT rather than VARCHAR(8000): Postgres stores
    -- both the same way, and a length cap here would reject an update the
    -- source accepted.
    content          TEXT,
    state            customer_engagement_status_update_state_enum,
    frequency        customer_engagement_status_update_frequency_enum,
    scope            customer_engagement_status_update_scope_enum,
    visibility       customer_engagement_status_update_visibility_enum,
    -- Start of the week this update covers. DATE for the same reason as above.
    cycle_start_date DATE,
    published_date   TIMESTAMPTZ,
    -- string(2500) upstream: a free-text recipient list, not a reference.
    mailing_list     VARCHAR(2500)
);

-- The lookup is (engagement, author, cycle) -- all three together, once per
-- allocation row -- so one composite index serves it rather than three.
CREATE INDEX IF NOT EXISTS idx_ce_status_update_lookup ON customer_engagement_status_update (engagement_id, author_id, cycle_start_date);
CREATE INDEX IF NOT EXISTS idx_ce_status_update_cycle  ON customer_engagement_status_update (cycle_start_date);
CREATE INDEX IF NOT EXISTS idx_ce_status_update_alloc  ON customer_engagement_status_update (allocation_id);
