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

-- sla.duration is a one-time snapshot of sla_policy.duration, copied in at
-- clock registration (registerClockExec) and never re-read afterward. That
-- was fine as long as sla_policy.duration itself was always trustworthy at
-- the moment a clock registered -- but a real production incident showed
-- it isn't: an external sync intermittently wrote a transient, wrong
-- duration into sla_policy (see entity-service/CLAUDE.md's own note on this
-- incident for the full story), and any clock that happened to register
-- during that window permanently baked the wrong value into its own
-- sla.duration, with no way to self-correct once sla_policy.duration was
-- fixed moments later.
--
-- sla_live (migration 0204/0245/0268) is this engine's single live-read
-- surface for elapsed/remaining/breach figures, and it computed every one
-- of those from s.duration -- exactly the column this incident proved can
-- be permanently wrong. This migration switches sla_live to compute from
-- sla_policy.duration (joined fresh on every read, via sla_policy_id)
-- instead: an sla row no longer carries its own frozen target at all for
-- live purposes, so a bad snapshot from the sync incident above (or any
-- future recurrence of it) can only ever be wrong for as long as
-- sla_policy.duration itself is wrong, and self-corrects the moment that
-- is fixed -- no backfill of already-registered clocks is ever needed
-- again for this specific failure mode.
--
-- LEFT JOIN, not INNER: sla_policy_id is NOT NULL and sla_policy rows are
-- never deleted in practice, but a defensive LEFT JOIN means an
-- unexpectedly orphaned row simply falls through to this view's own
-- existing ELSE branches (the frozen business_elapsed_percentage/
-- business_duration/remaining_business_duration/has_breached/stage columns)
-- instead of disappearing from the view entirely. The join is by
-- sla_policy's own primary key, so it can never multiply a row.
--
-- sla.duration itself is NOT dropped here -- it's still selected via
-- s.*, and still written by registerClockExec -- it remains a useful
-- historical record of what a clock's target was understood to be at
-- registration, it's just no longer trusted for any LIVE calculation.
--
-- CREATE OR REPLACE is safe here (unlike migration 0268's own DROP+CREATE):
-- the view's output column list and types are unchanged, only the source
-- expression feeding four already-existing computed columns changes.
CREATE OR REPLACE VIEW sla_live AS
SELECT s.*,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
        THEN GREATEST(0,
                EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))
                / NULLIF(EXTRACT(EPOCH FROM sp.duration), 0) * 100
             )
        ELSE s.business_elapsed_percentage
    END AS live_elapsed_percentage,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL
        THEN GREATEST(NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'), INTERVAL '0')
        ELSE s.business_duration
    END AS live_business_duration,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
        THEN GREATEST(sp.duration - (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')), INTERVAL '0')
        ELSE s.remaining_business_duration
    END AS live_remaining_business_duration,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))) >= EXTRACT(EPOCH FROM sp.duration)
        THEN TRUE
        ELSE COALESCE(s.has_breached, FALSE)
    END AS live_has_breached,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND sp.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))) >= EXTRACT(EPOCH FROM sp.duration)
        THEN 'BREACHED'::sla_stage_enum
        ELSE s.stage
    END AS live_stage
FROM sla s
LEFT JOIN sla_policy sp ON sp.id = s.sla_policy_id;
