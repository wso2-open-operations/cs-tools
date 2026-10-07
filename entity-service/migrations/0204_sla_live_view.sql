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

-- Read-time replacement for SLAEngineRecomputeWorker (removed in this same
-- change), which used to keep a source='CSM' "sla" row's displayed
-- business_elapsed_percentage/has_breached/business_duration/
-- remaining_business_duration/stage current by rewriting every active row's
-- columns on a 45s timer -- a continuous Postgres write with no bearing on
-- alerting (csm-notification-service's own Redis engine fires breach alerts
-- independently of this table).
--
-- sla_live reproduces that worker's own RecomputeActive formula (same flat
-- wall-clock math vs. start_on/duration, including its documented
-- pause-accounting crudeness -- see that function's own doc comment; this
-- changes WHEN the number is computed, not what it computes) live, in the
-- SELECT, for a row still IN_PROGRESS/BREACHED. Every other stage's row
-- (PAUSED/COMPLETED/ACHIEVED/CANCELLED) is already frozen at whatever
-- SetPaused/CompleteClock last wrote and needs no recompute -- the view
-- simply passes those columns through.
--
-- Every reader of business_elapsed_percentage/has_breached/business_duration/
-- remaining_business_duration/stage off "sla" (GET /sla-status,
-- POST /task-slas/search, GET /task-slas/{id}, the case/incident
-- SLA-breach search filters) reads sla_live's live_* columns instead.
-- Nothing writes to this view; RegisterClock/CompleteClock/SetPaused/
-- ReviseClocks keep writing the real "sla" table underneath, unchanged.
CREATE OR REPLACE VIEW sla_live AS
SELECT s.*,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
        THEN GREATEST(0, EXTRACT(EPOCH FROM (NOW() - s.start_on)) / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100)
        ELSE s.business_elapsed_percentage
    END AS live_elapsed_percentage,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL
        THEN NOW() - s.start_on
        ELSE s.business_duration
    END AS live_business_duration,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
        THEN GREATEST(s.duration - (NOW() - s.start_on), INTERVAL '0')
        ELSE s.remaining_business_duration
    END AS live_remaining_business_duration,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on)) >= EXTRACT(EPOCH FROM s.duration)
        THEN TRUE
        ELSE COALESCE(s.has_breached, FALSE)
    END AS live_has_breached,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on)) >= EXTRACT(EPOCH FROM s.duration)
        THEN 'BREACHED'::sla_stage_enum
        ELSE s.stage
    END AS live_stage
FROM sla s;
