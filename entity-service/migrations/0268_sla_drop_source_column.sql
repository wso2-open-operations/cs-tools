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

-- sla.source (migration 0134) existed to keep this engine's own clock rows
-- ('CSM') from colliding with a real, independent, continuously-running
-- external sync job that also wrote clock rows into this same table
-- ('SERVICENOW'). At the cutover this migration ships with, that sync job
-- stops writing SLA rows into this table at all -- there is only ever one
-- writer of "sla" from this point on, so there is nothing left for this
-- column to distinguish. Every row already in the table at cutover
-- (regardless of what it was previously tagged) is simply adopted in place:
-- its own start_on/duration/pause_duration already reflect genuine elapsed
-- and paused time, so this engine's own stop/pause/resume/cancel evaluation
-- picks it up and continues it correctly with no separate backfill step.
--
-- sla_policy.source is a SEPARATE, unrelated column (also from migration
-- 0134) and is NOT touched here -- it distinguishes a real previous-system-
-- synced POLICY DEFINITION from anything else, which remains a meaningful
-- distinction regardless of who writes clock rows.
--
-- sla_live (migration 0204/0245) is defined as `SELECT s.*, ...` and so
-- depends on every column of "sla" individually, source included -- it must
-- be dropped and recreated (identical to migration 0245's own definition,
-- minus the dropped column) rather than just CREATE OR REPLACE'd, since
-- REPLACE cannot change a view's underlying input columns. idx_sla_source
-- (migration 0134) is dropped automatically by DROP COLUMN; no explicit
-- DROP INDEX is needed.
--
-- IF EXISTS on both the view drop and the column drop, so a direct
-- reapplication of this file succeeds as a no-op on those two statements
-- instead of failing outright once they've already run once -- the same
-- "safe to re-run" convention this repo's migrations already follow.
--
-- Pre-flight safety check: this migration adopts every existing "sla" row
-- in place regardless of its old source tag (see above), which is only
-- correct if no (work_item, target) pair has an ACTIVE row from BOTH
-- sources at once -- migration 0134 never added a uniqueness constraint
-- that would have prevented that. The application code picks one row per
-- (work_item, target) by most-recently-started/updated once this column is
-- gone (sla_status_repo.go's own DISTINCT ON), with no way left to tell the
-- two apart, so an undetected overlap would silently adopt whichever one
-- happens to look newer and abandon the other. The DO block below checks
-- for this (only while the column still exists, so a re-run after a
-- successful first pass is a no-op here too) and refuses to proceed if any
-- overlap is found -- resolve which row to keep and which to cancel by hand
-- before retrying.
BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$
DECLARE
    overlap_count integer;
BEGIN
    -- to_regclass/pg_attribute, not information_schema.columns -- resolves
    -- "sla" the same way any ordinary query would (via the session's own
    -- search_path), so this check works regardless of which schema a given
    -- client session happens to default to, unlike matching against a
    -- literal current_schema() value.
    IF EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = to_regclass('sla') AND attname = 'source' AND NOT attisdropped
    ) THEN
        SELECT COUNT(*) INTO overlap_count FROM (
            SELECT s.work_item_id, sp.target
            FROM sla s
            JOIN sla_policy sp ON sp.id = s.sla_policy_id
            WHERE s.is_active AND sp.target IS NOT NULL
            GROUP BY s.work_item_id, sp.target
            HAVING COUNT(DISTINCT s.source) > 1
        ) dup_pairs;

        IF overlap_count > 0 THEN
            RAISE EXCEPTION 'migration 0268: % work_item/target pair(s) have an active sla row from both source values -- resolve which row to keep (cancel the other) before dropping sla.source; see this migration''s own doc comment', overlap_count;
        END IF;
    END IF;
END $$;

DROP VIEW IF EXISTS sla_live;

ALTER TABLE sla DROP COLUMN IF EXISTS source;

CREATE OR REPLACE VIEW sla_live AS
SELECT s.*,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
        THEN GREATEST(0,
                EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))
                / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
             )
        ELSE s.business_elapsed_percentage
    END AS live_elapsed_percentage,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL
        THEN GREATEST(NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'), INTERVAL '0')
        ELSE s.business_duration
    END AS live_business_duration,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
        THEN GREATEST(s.duration - (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')), INTERVAL '0')
        ELSE s.remaining_business_duration
    END AS live_remaining_business_duration,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))) >= EXTRACT(EPOCH FROM s.duration)
        THEN TRUE
        ELSE COALESCE(s.has_breached, FALSE)
    END AS live_has_breached,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))) >= EXTRACT(EPOCH FROM s.duration)
        THEN 'BREACHED'::sla_stage_enum
        ELSE s.stage
    END AS live_stage
FROM sla s;

COMMIT;
