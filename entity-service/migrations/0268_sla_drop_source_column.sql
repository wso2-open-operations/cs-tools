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
BEGIN;
SET LOCAL lock_timeout = '5s';

DROP VIEW sla_live;

ALTER TABLE sla DROP COLUMN source;

CREATE VIEW sla_live AS
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
