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

-- sla_live's own IN_PROGRESS/BREACHED formula (migration 0272) is flat
-- wall-clock time -- correct for a '24 x 7' policy, wrong for 'WSO2
-- Business days': it never skips Saturday/Sunday. internal/businessdays'
-- own Go package (Engine's read paths: GetClockState,
-- SearchActiveSLAStatuses, TaskSlaRepository) already corrects for this
-- AFTER reading the view, in Go -- but two callers never go through that
-- correction: the case-search and incident-search SLA filters
-- (slaPercentPredicate/slaBreachedPredicate, internal/repository/
-- sla_filter_sql.go) read sla_live's own live_elapsed_percentage/
-- live_has_breached directly, with no Go-side correction layered on top at
-- all. A live comparison against the real, currently-running 'WSO2 Business
-- days' clocks found every one of them disagreeing with the Go-corrected
-- figure by more than a point, typically by 25-35 points -- cases near a
-- search's percentage bounds were wrongly included or excluded, and a saved
-- "SLA at risk" view returned mostly wrong results.
--
-- The case/incident search filters can't apply the Go-side correction the
-- way the three EXISTING callers do: those are small, bounded reads (the
-- admin SLA table caps at ~5,500 active clocks; the detail endpoints read
-- one clock at a time) that can afford to fetch-then-correct-then-paginate
-- in Go. A case/incident search runs over the full case table with real
-- SQL pagination -- correcting a few thousand matching SLA rows in Go
-- before applying LIMIT/OFFSET would mean fetching every potentially
-- matching row up front, the exact thing SearchActiveSLAStatuses' own doc
-- comment already flags as unaffordable for a genuinely large, open-ended
-- result set. So the fix has to move into sla_live's own SQL instead of
-- adding a fourth Go-side correction layer: see business_seconds_elapsed
-- below and the view change that uses it.
--
-- No BEGIN/COMMIT here -- this repo's migrations are applied with plain
-- autocommit, never wrapped in an explicit transaction (CREATE OR REPLACE
-- FUNCTION/VIEW are each already safe to re-run on their own).

-- business_seconds_elapsed mirrors internal/businessdays.Elapsed exactly: a
-- straight, line-by-line SQL port of that Go function's own day-stepping
-- loop, not a reimplementation from first principles -- the two must keep
-- agreeing, so changing one without the other is a bug. Returns how many
-- seconds of [p_start, p_end) fall on a Monday-Friday calendar day in
-- America/New_York, skipping Saturday/Sunday entirely; p_end at or before
-- p_start returns 0, never negative (same "clock skew must not go
-- negative" posture as sla_engine_repo.go's own GREATEST(0, ...) floors).
--
-- Day boundaries are computed by converting to a naive "wall clock in
-- America/New_York" timestamp (`AT TIME ZONE`), truncating and stepping by
-- a full calendar day there, then converting that naive boundary BACK to a
-- real instant via `AT TIME ZONE` again -- Postgres re-resolves the zone's
-- real UTC offset for that specific local date on each conversion, which is
-- what makes a DST transition day correctly span 23 or 25 real hours
-- instead of a fixed 24 -- the exact same correctness property
-- internal/businessdays.startOfNextDay's own doc comment explains for why
-- it uses time.Date instead of Add(24*time.Hour).
--
-- IMMUTABLE/PARALLEL SAFE: pure function of its two arguments, touches no
-- table, calls now() nowhere -- same reasoning migration 100018 already
-- established for this schema's other read-only helper functions, so a
-- query using this inside sla_live's WHERE/SELECT stays eligible for a
-- parallel plan.
--
-- Validated against internal/businessdays.Elapsed's own output, run inside
-- a transaction rolled back afterward (never left in the database): a
-- same-weekday range, a range spanning one weekend, a multi-week range
-- spanning three weekends, and -- the case this port exists to get right --
-- a week each crossing the spring-forward and fall-back DST transitions.
-- Every case matched exactly. Also checked the full corrected-percentage
-- formula (this function's output minus a clock's own completed
-- business_pause_duration, divided by its policy's duration) against real,
-- currently-running 'WSO2 Business days' clocks already known to disagree
-- with sla_live's own uncorrected figure -- the corrected result matched
-- the same independently-verified figure the Go engine's own correction
-- already produces for the identical clocks.
CREATE OR REPLACE FUNCTION business_seconds_elapsed(p_start TIMESTAMPTZ, p_end TIMESTAMPTZ)
RETURNS DOUBLE PRECISION
LANGUAGE plpgsql
IMMUTABLE
PARALLEL SAFE
AS $$
DECLARE
    cur_instant TIMESTAMPTZ;
    cur_local TIMESTAMP;
    next_day_instant TIMESTAMPTZ;
    segment_end TIMESTAMPTZ;
    total DOUBLE PRECISION := 0;
    iso_dow INT;
BEGIN
    IF p_end <= p_start THEN
        RETURN 0;
    END IF;

    cur_instant := p_start;
    WHILE cur_instant < p_end LOOP
        cur_local := cur_instant AT TIME ZONE 'America/New_York';
        next_day_instant := (date_trunc('day', cur_local) + INTERVAL '1 day') AT TIME ZONE 'America/New_York';
        segment_end := LEAST(next_day_instant, p_end);
        iso_dow := EXTRACT(ISODOW FROM cur_local); -- 1=Monday .. 7=Sunday, same local wall-clock date business_seconds_elapsed just resolved
        IF iso_dow BETWEEN 1 AND 5 THEN
            total := total + EXTRACT(EPOCH FROM (segment_end - cur_instant));
        END IF;
        cur_instant := segment_end;
    END LOOP;

    RETURN total;
END;
$$;

-- sla_live, re-created with one addition: for a still-running (IN_PROGRESS/
-- BREACHED) row whose policy schedule is 'WSO2 Business days', every live_*
-- column now derives from business_seconds_elapsed(start_on, NOW()) minus
-- the row's own already-completed business_pause_duration -- the identical
-- formula businessDayElapsedSnapshot (sla_engine_repo.go) already computes
-- in Go, just moved into the view itself so every reader (not only the
-- three that already apply Go's own correction) sees the same, correct
-- figure. Every other case (24x7 schedule, PAUSED/terminal stage regardless
-- of schedule) is completely unchanged -- the bd.elapsed_seconds lateral
-- join simply doesn't fire for them (its own WHERE clause below), so the
-- COALESCE in every CASE branch falls through to migration 0272's original
-- flat-wall-clock expression, byte-for-byte.
--
-- live_business_duration/live_remaining_business_duration are rebuilt via
-- make_interval(secs => ...) from the same elapsed-seconds figure that
-- feeds live_elapsed_percentage, rather than re-deriving them from a
-- separate flat NOW()-start_on expression -- a single source of truth per
-- row, computed once by the lateral join below, not the function called up
-- to four times per row.
--
-- live_has_breached/live_stage keep migration 0272's own, deliberate
-- asymmetry: only a row whose RAW s.stage is still 'IN_PROGRESS' is ever
-- live-bumped to BREACHED here -- a row whose stage is already stored as
-- 'BREACHED' keeps whatever s.has_breached/s.stage already say, exactly as
-- before.
--
-- bd.elapsed_seconds::numeric in the percentage branch below, not bare --
-- a CodeRabbit-caught, migration-breaking bug: business_seconds_elapsed
-- returns DOUBLE PRECISION, but live_elapsed_percentage's existing column
-- type (inherited from migration 0272's own EXTRACT(EPOCH FROM ...)
-- arithmetic, NUMERIC on Postgres 12+) is NUMERIC. An uncast
-- COALESCE(double precision, numeric-valued expression) resolves to double
-- precision, silently changing this view column's own output type --
-- which CREATE OR REPLACE VIEW categorically refuses (a view's column
-- types must match exactly; changing one needs DROP + CREATE, which
-- migration 0272's own doc comment explicitly avoided). The cast is only
-- needed here: live_business_duration/live_remaining_business_duration
-- feed the same COALESCE into make_interval(secs => ...), whose own
-- double-precision parameter coerces either type at the call boundary, and
-- live_has_breached/live_stage only ever use the COALESCE inside a boolean
-- comparison, which compares numeric and double precision directly with no
-- output-type consequence.
CREATE OR REPLACE VIEW sla_live AS
SELECT s.*,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
        THEN COALESCE(bd.elapsed_seconds::numeric, GREATEST(0, EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))))
             / NULLIF(EXTRACT(EPOCH FROM sp.duration), 0) * 100
        ELSE s.business_elapsed_percentage
    END AS live_elapsed_percentage,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL
        THEN make_interval(secs => COALESCE(bd.elapsed_seconds, GREATEST(EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))), 0)))
        ELSE s.business_duration
    END AS live_business_duration,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
        THEN GREATEST(sp.duration - make_interval(secs => COALESCE(bd.elapsed_seconds, GREATEST(EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))), 0))), INTERVAL '0')
        ELSE s.remaining_business_duration
    END AS live_remaining_business_duration,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
             AND COALESCE(bd.elapsed_seconds, EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))) >= EXTRACT(EPOCH FROM sp.duration)
        THEN TRUE
        ELSE COALESCE(s.has_breached, FALSE)
    END AS live_has_breached,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
             AND COALESCE(bd.elapsed_seconds, EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))) >= EXTRACT(EPOCH FROM sp.duration)
        THEN 'BREACHED'::sla_stage_enum
        ELSE s.stage
    END AS live_stage
FROM sla s
LEFT JOIN sla_policy sp ON sp.id = s.sla_policy_id
LEFT JOIN LATERAL (
    SELECT GREATEST(0, business_seconds_elapsed(s.start_on, NOW()) - COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0)) AS elapsed_seconds
    WHERE sp.schedule = 'WSO2 Business days' AND s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL
) bd ON TRUE;
