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

-- The ServiceNow support-rotation roster: who covers which shift on which day,
-- plus the leave that makes someone unavailable. Source of the "Dispatch Email
-- Notification for Weekend Team" / "... for Support Rotation" flows, via the
-- TeamScheduleLoader script include.
--
-- Three tables because ServiceNow uses three, and this repo mirrors rather than
-- reshapes: a span points at a schedule, and a separate binding says whose
-- schedule that is. The indirection exists upstream because cmn_schedule_span
-- is a generic platform table; keeping it means an imported span's identity and
-- references survive the move unchanged.
--
--   cmn_schedule       -> schedule
--   agent_events       -> user_schedule
--   cmn_schedule_span  -> schedule_span

-- cmn_schedule. NOT purely per-person containers: of 929 rows upstream, the
-- sample includes "Support - LK - Week days", "Lead", "Integration CCC-2" and
-- "weekend 9 - 9" alongside the "<User Name> Personal Schedule" rows that
-- UpdateTeamSchduleForRotation creates.
--
-- time_zone is load-bearing, not decoration: every schedule upstream is
-- Asia/Kolkata, and schedule_span.start_on/end_on below are wall-clock times IN
-- THIS ZONE rather than instants. The zone lives here, once per schedule,
-- exactly as it does upstream.
-- Wrapped in a transaction deliberately. `make migrate` runs each file with
-- `psql -f`, not `--single-transaction`, so every statement would otherwise
-- commit on its own. For a file that drops and recreates a trigger that means
-- a window where the table has no trigger at all, and a write landing in it
-- produces no outbox row and no notice -- silently, with nothing to retry.
--
-- Postgres makes DDL transactional, so the swap becomes one step: a concurrent
-- write waits for the lock instead of slipping through the gap. It also makes
-- the whole migration all-or-nothing, rather than half-applied and unrecorded
-- in csm_migration_applied_migration if a later statement fails.
BEGIN;

CREATE TABLE IF NOT EXISTS schedule (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    -- Nullable, unlike most tables here (21 NOT NULL vs 7 nullable, "user"
    -- itself among the latter). sys_created_by/sys_updated_by are inherited
    -- columns on these platform tables and were NOT confirmed populated during
    -- discovery -- the dictionary query that shaped this migration only returns
    -- columns declared on the table itself. NOT NULL here would fail the whole
    -- row for metadata the upsert never reads, so it buys nothing and risks the
    -- load. created_on/updated_on stay NOT NULL: the merge predicate
    -- (EXCLUDED.updated_on >= t.updated_on) compares them, and a NULL there
    -- would make the row never update.
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    name VARCHAR(255),
    type VARCHAR(100),
    time_zone VARCHAR(64),
    read_only BOOLEAN
);

-- agent_events: binds a user to a schedule.
--
-- NO UNIQUE on schedule_id, deliberately, even though
-- TeamScheduleLoader._get_personal_scheduleIds builds its map KEYED BY
-- personal_schedule and would therefore resolve an arbitrary user if two
-- bindings ever shared one schedule. The loader upserts with ON CONFLICT (id)
-- -- a PK collision is absorbed, but any OTHER unique violation fails the row
-- outright. On a replica, refusing to load upstream data is worse than
-- reproducing its ambiguity: a duplicate here should surface as a visible data
-- problem, not as a table that silently stops syncing. A reader resolving a
-- schedule to a user must pick deterministically (e.g. lowest id) rather than
-- assume uniqueness. One user holding several schedules is normal and is what
-- the data shows -- one sampled user appears against four.
CREATE TABLE IF NOT EXISTS user_schedule (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    -- Nullable, unlike most tables here (21 NOT NULL vs 7 nullable, "user"
    -- itself among the latter). sys_created_by/sys_updated_by are inherited
    -- columns on these platform tables and were NOT confirmed populated during
    -- discovery -- the dictionary query that shaped this migration only returns
    -- columns declared on the table itself. NOT NULL here would fail the whole
    -- row for metadata the upsert never reads, so it buys nothing and risks the
    -- load. created_on/updated_on stay NOT NULL: the merge predicate
    -- (EXCLUDED.updated_on >= t.updated_on) compares them, and a NULL there
    -- would make the row never update.
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    schedule_id UUID NOT NULL REFERENCES schedule(id) ON DELETE CASCADE,
    active BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_user_schedule_user_id ON user_schedule (user_id);
CREATE INDEX IF NOT EXISTS idx_user_schedule_schedule_id ON user_schedule (schedule_id);

-- cmn_schedule_span: the roster rows themselves, and the leave rows.
--
-- start_on/end_on are TIMESTAMP WITHOUT TIME ZONE deliberately. Upstream these
-- are schedule_date_time values ("20250805T000000") interpreted in the owning
-- schedule's time_zone; TeamScheduleUtils.adjustTime snaps them to 00:00:00 /
-- 23:59:59 of a LOCAL day. A TIMESTAMPTZ column would label those digits UTC
-- and shift every span by the zone offset -- enough to move a whole-day span
-- onto the neighbouring day, which is exactly what the rotation lookup keys on.
-- See the schedule_date_time transform in internal/transform.
--
-- NO RECURRENCE COLUMNS, on evidence rather than simplification: repeat_type is
-- empty on all 891 rotation/leave spans upstream, which makes days_of_week /
-- month / monthly_type / yearly_type / float_day / float_week / repeat_count /
-- repeat_until inert platform defaults (1, 'dom', 'doy', '00000000'). Mirroring
-- them would copy noise. If recurrence is ever used, this needs a column and
-- the mapping needs to expand the series.
--
-- user_id and team_id mirror the span's own direct references (24 and 12 of 891
-- rows respectively), which bypass the schedule -> user_schedule hop entirely.
-- A minority path, but a real one -- dropping it would lose those rows' owner.
CREATE TABLE IF NOT EXISTS schedule_span (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    -- Nullable, unlike most tables here (21 NOT NULL vs 7 nullable, "user"
    -- itself among the latter). sys_created_by/sys_updated_by are inherited
    -- columns on these platform tables and were NOT confirmed populated during
    -- discovery -- the dictionary query that shaped this migration only returns
    -- columns declared on the table itself. NOT NULL here would fail the whole
    -- row for metadata the upsert never reads, so it buys nothing and risks the
    -- load. created_on/updated_on stay NOT NULL: the merge predicate
    -- (EXCLUDED.updated_on >= t.updated_on) compares them, and a NULL there
    -- would make the row never update.
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    schedule_id UUID NOT NULL REFERENCES schedule(id) ON DELETE CASCADE,
    span_type VARCHAR(50),
    start_on TIMESTAMP,
    end_on TIMESTAMP,
    all_day BOOLEAN,
    name VARCHAR(255),
    notes TEXT,
    user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    team_id UUID REFERENCES team(id) ON DELETE SET NULL
);

-- The rotation lookup: spans of a given type covering a given local day.
-- span_type leads because it is the equality term; the range columns follow.
CREATE INDEX IF NOT EXISTS idx_schedule_span_type_window ON schedule_span (span_type, start_on, end_on);
CREATE INDEX IF NOT EXISTS idx_schedule_span_schedule_id ON schedule_span (schedule_id);
CREATE INDEX IF NOT EXISTS idx_schedule_span_user_id ON schedule_span (user_id);

COMMIT;
