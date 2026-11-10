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

-- change_request.closed_on / closed_by_user_id (0043) were never written by the
-- CSM's own writers: the PATCH path, the customer stage's outcome and the GitHub
-- sync moved a change request to Closed or Rollback and left both columns empty
-- (only rows the previous system's sync loaded carry them). The writers now stamp
-- them on entering either state (changeRequestClosureSets, entity-service); this
-- fills the rows completed before that, from the change request's own history.
--
-- Where the time comes from: event_outbox (0051) has a row for every UPDATE of
-- change_request that changed something, with the state's from / to when the
-- state moved and occurred_on = the writing transaction's NOW(). The first move
-- INTO Closed / Rollback from any other state is the completion (both are final,
-- so there is normally exactly one).
--
-- Who: the outbox does not record the actor. The PATCH path stamps
-- work_item.updated_on = NOW() and updated_by = the actor in the same
-- transaction, so when the work item still carries that exact instant, its
-- updated_by is who completed the change. Anything written to the work item
-- since (a work note) moves updated_on, and then no closer is recorded rather
-- than a guess. A customer's review answer and the GitHub sync never write the
-- work item, so those keep no closer.
--
-- A change request completed before the outbox existed, or with no recorded
-- transition, is left alone. Re-runnable: only rows still in Closed / Rollback
-- with no closed_on are written, and a second run finds none. The UPDATE itself
-- goes through change_request_outbox like any write; its rows carry only these
-- two columns, which no consumer reacts to.
--
-- RUN IT AGAIN ONCE THE NEW CODE IS FULLY ROLLED OUT. `make migrate` runs before
-- the rollout, and pods still on the old code keep completing change requests
-- without closed_on until they are replaced; this file is recorded as applied and
-- `make migrate` never runs it again. Re-running it by hand afterwards fills those
-- (and touches nothing else):
--   psql "$conn" -v ON_ERROR_STOP=1 -f migrations/0283_change_request_closed_on_backfill.sql
--
-- change_request and work_item are row-level-secured with FORCE (100006, 100008):
-- a migrating role without BYPASSRLS that is not flagged internal sees no row, and
-- the UPDATE would write nothing without an error. So the session is flagged
-- internal for it, as the application's own writes are, and cleared afterwards,
-- as 0193 does (session-level, not LOCAL: a migration runs in plain autocommit,
-- where a LOCAL setting would be gone before the UPDATE runs).

SELECT set_config('app.is_internal', 'true', false);

WITH first_completion AS (
    SELECT DISTINCT ON (eo.entity_id) eo.entity_id, eo.occurred_on
      FROM event_outbox eo
     WHERE eo.entity_type = 'change_request'
       AND eo.changes -> 'state' ->> 'to' IN ('CLOSED', 'ROLLBACK')
       AND COALESCE(eo.changes -> 'state' ->> 'from', '') NOT IN ('CLOSED', 'ROLLBACK')
     ORDER BY eo.entity_id, eo.occurred_on, eo.id
)
UPDATE change_request cr
   SET closed_on = fc.occurred_on,
       closed_by_user_id = COALESCE(cr.closed_by_user_id, (
           SELECT u.id
             FROM work_item wi
             JOIN "user" u ON LOWER(u.email) = LOWER(wi.updated_by)
            WHERE wi.id = cr.id AND wi.updated_on = fc.occurred_on
            ORDER BY u.id
            LIMIT 1))
  FROM first_completion fc
 WHERE cr.id = fc.entity_id
   AND cr.closed_on IS NULL
   AND cr.state IN ('CLOSED', 'ROLLBACK');

SELECT set_config('app.is_internal', '', false);
