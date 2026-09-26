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

-- Team Schedule: who from CRE and SRE is working, when, and in which
-- escalation tier. There is no ServiceNow equivalent -- this is portal-native
-- data, like announcement_requests -- so these tables are the system of
-- record, not a mirror of one.
--
-- The rota is AUTHORED in one clock (IST, the clock the shift windows are
-- written in) and READ in whatever clock the viewer is in. That is why
-- schedule_shift stores minutes-of-day against an authoring_time_zone while
-- schedule_assignment stores the resolved absolute instants: the catalogue
-- stays human-editable, and "who is on duty at 03:14 UTC" stays a plain
-- indexed range query that no client has to re-derive.

DO $$ BEGIN
    CREATE TYPE schedule_shift_family_enum AS ENUM ('CRE', 'SRE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- L1/L2/L3 are escalation tiers, not seniority. A shift with tier NULL is
-- ordinary working hours (CRE regular hours, SRE "others in the zone").
DO $$ BEGIN
    CREATE TYPE schedule_tier_enum AS ENUM ('L1', 'L2', 'L3');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Weekday and weekend are different shapes of day, not different windows of
-- the same shape: SRE runs three zones on a weekday and two at the weekend.
DO $$ BEGIN
    CREATE TYPE schedule_day_scope_enum AS ENUM ('WEEKDAY', 'WEEKEND', 'ANY');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- How a row got there. 'GENERATED' rows came from the round-robin allocator
-- and may be regenerated in bulk; 'MANUAL' and 'SWAP' rows were put there by
-- a lead and must survive any regeneration.
DO $$ BEGIN
    CREATE TYPE schedule_source_enum AS ENUM ('GENERATED', 'MANUAL', 'SWAP', 'IMPORTED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Everything that takes an engineer out of the rota for whole days at a time.
DO $$ BEGIN
    CREATE TYPE schedule_absence_kind_enum AS ENUM (
        'ANNUAL_LEAVE', 'LIEU_LEAVE', 'RND_ALLOCATION', 'CUSTOMER_ALLOCATION',
        'ONBOARDING', 'EXCLUDED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── The catalogue ─────────────────────────────────────────────────────────

-- An SRE time zone. Codes are TZ1/TZ2/TZ3 on a weekday; at the weekend the
-- day collapses to two, so each weekday zone names the weekend zone that
-- absorbs it (TZ1->TZ1, TZ2->TZ1, TZ3->TZ2). Without that mapping, a
-- Saturday's 00:00-06:00 -- which belongs to Friday's TZ3 crew -- cannot be
-- attributed to any weekend zone at all.
CREATE TABLE IF NOT EXISTS schedule_zone (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    code                VARCHAR(8) NOT NULL UNIQUE,
    label               VARCHAR(50) NOT NULL,
    weekend_zone_id     UUID REFERENCES schedule_zone(id) ON DELETE SET NULL,
    sort_order          SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE
);

-- A named window of the working day. Minutes are counted from midnight in
-- authoring_time_zone; an end past 1440 runs into the next day, so the SRE
-- night block 21:00-06:00 is stored as 1260 -> 1800 rather than as two rows
-- that a reader has to stitch back together.
CREATE TABLE IF NOT EXISTS schedule_shift (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    code                VARCHAR(32) NOT NULL UNIQUE,
    label               VARCHAR(100) NOT NULL,
    family              schedule_shift_family_enum NOT NULL,
    zone_id             UUID REFERENCES schedule_zone(id) ON DELETE RESTRICT,
    tier                schedule_tier_enum,
    day_scope           schedule_day_scope_enum NOT NULL DEFAULT 'WEEKDAY',
    start_minute        SMALLINT NOT NULL,
    end_minute          SMALLINT NOT NULL,
    authoring_time_zone VARCHAR(64) NOT NULL DEFAULT 'Asia/Colombo',
    is_on_call          BOOLEAN NOT NULL DEFAULT FALSE,
    is_escalation       BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order          SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    crosses_midnight    BOOLEAN GENERATED ALWAYS AS (end_minute > 1440) STORED,
    CONSTRAINT schedule_shift_start_minute_check
        CHECK (start_minute >= 0 AND start_minute < 1440),
    CONSTRAINT schedule_shift_end_minute_check
        CHECK (end_minute > start_minute AND end_minute <= 2880),
    -- only SRE works in time zones; and a window that hosts an escalation
    -- tier must say which zone it covers, or "who is L1 right now" has no
    -- answer. SRE regular hours carry no zone of their own -- the engineer's
    -- zone for that day rides on the assignment, not on the window.
    CONSTRAINT schedule_shift_zone_family_check
        CHECK (zone_id IS NULL OR family = 'SRE'),
    CONSTRAINT schedule_shift_escalation_zone_check
        CHECK (NOT is_escalation OR zone_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_schedule_shift_family ON schedule_shift (family);
CREATE INDEX IF NOT EXISTS idx_schedule_shift_zone_id ON schedule_shift (zone_id);

-- ── The facts ─────────────────────────────────────────────────────────────

-- One engineer, one rota day, one window. Every assignment is stored, not
-- derived: the roster is evidence of who was responsible at a given moment,
-- so a change to the allocator must never rewrite what already happened.
--
-- rota_date is the day the CREW is rostered for, not the calendar date of
-- every hour worked. A Monday 21:00-06:00 block carries rota_date = Monday
-- even though six of its hours fall on Tuesday -- that is what makes
-- "Monday's night crew" answerable, and it matches how the handover is run.
--
-- starts_at/ends_at are resolved from shift + rota_date + authoring_time_zone
-- at write time. They are the only thing a point-in-time lookup needs to
-- touch, and they stay correct across DST because they are absolute instants.
--
-- team_key is the CSM_TEAM_REGISTRY key (the BFF's own team vocabulary);
-- team_id is the entity-service team row when one exists. Both are kept: the
-- registry is env config with no FK to offer, and an engineer who moves team
-- must not retroactively change which team covered a past shift.
CREATE TABLE IF NOT EXISTS schedule_assignment (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    user_id             UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    team_id             UUID REFERENCES team(id) ON DELETE SET NULL,
    team_key            VARCHAR(64) NOT NULL,
    shift_id            UUID NOT NULL REFERENCES schedule_shift(id) ON DELETE RESTRICT,
    zone_id             UUID REFERENCES schedule_zone(id) ON DELETE RESTRICT,
    tier                schedule_tier_enum,
    rota_date           DATE NOT NULL,
    starts_at           TIMESTAMPTZ NOT NULL,
    ends_at             TIMESTAMPTZ NOT NULL,
    is_on_call          BOOLEAN NOT NULL DEFAULT FALSE,
    source              schedule_source_enum NOT NULL DEFAULT 'MANUAL',
    note                TEXT,
    CONSTRAINT schedule_assignment_window_check CHECK (ends_at > starts_at),
    CONSTRAINT schedule_assignment_unique_slot UNIQUE (user_id, rota_date, shift_id)
);

CREATE INDEX IF NOT EXISTS idx_schedule_assignment_rota_date
    ON schedule_assignment (rota_date);
CREATE INDEX IF NOT EXISTS idx_schedule_assignment_user_date
    ON schedule_assignment (user_id, rota_date);
CREATE INDEX IF NOT EXISTS idx_schedule_assignment_team_date
    ON schedule_assignment (team_key, rota_date);
CREATE INDEX IF NOT EXISTS idx_schedule_assignment_zone_tier_date
    ON schedule_assignment (zone_id, tier, rota_date);

-- "Who is on duty at this instant" -- the query an alert escalation makes,
-- and the one the day view's now-line makes. A GiST index over the window as
-- a range answers it directly; range types carry their own GiST opclass, so
-- this needs no extension.
CREATE INDEX IF NOT EXISTS idx_schedule_assignment_window
    ON schedule_assignment USING GIST (tstzrange(starts_at, ends_at, '[)'));

-- Whole days an engineer is not available to the rota: leave, or time given
-- to R&D or to a named customer. Stored as a range because that is how it is
-- granted and how a lead marks it -- one row for "12-19 March", not eight.
--
-- Weekends are deliberately NOT excluded here by a constraint: a leave range
-- may legitimately span one. Which days inside the range actually count is a
-- service-layer rule (AL/LL cannot be claimed against a weekend), applied
-- when the range is expanded into days.
CREATE TABLE IF NOT EXISTS schedule_absence (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    user_id             UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    team_key            VARCHAR(64) NOT NULL,
    kind                schedule_absence_kind_enum NOT NULL,
    starts_on           DATE NOT NULL,
    ends_on             DATE NOT NULL,
    note                TEXT,
    approved_by         VARCHAR(255),
    approved_on         TIMESTAMPTZ,
    CONSTRAINT schedule_absence_range_check CHECK (ends_on >= starts_on)
);

CREATE INDEX IF NOT EXISTS idx_schedule_absence_user
    ON schedule_absence (user_id, starts_on, ends_on);
CREATE INDEX IF NOT EXISTS idx_schedule_absence_team
    ON schedule_absence (team_key, starts_on);
CREATE INDEX IF NOT EXISTS idx_schedule_absence_span
    ON schedule_absence USING GIST (daterange(starts_on, ends_on, '[]'));
