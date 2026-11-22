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

-- "Cloud Incident - High Priority" is the one incident SLA policy (a 4 h,
-- 24 x 7 resolution clock). Its conditions were still the raw text the
-- previous system's sync wrote -- numeric priority and state codes, a
-- lowercase category, the "^EQ" terminator, and two business_service
-- clauses carrying raw service ids -- while every case policy row already
-- spells its conditions in this engine's own decoded vocabulary (enum
-- labels, no terminator; see 0269 for the shape).
--
-- This rewrites the row the same way, with one deliberate change of
-- meaning: the service clauses are dropped. The clock now starts for ANY
-- service whose incident is Critical or High priority with category
-- Service Interruption, not only for the two services the raw row named,
-- and a service change no longer cancels it. Stop (On Hold, Resolved,
-- Closed count as met) and the remaining cancel branches (Canceled, the
-- category changing, the priority dropping below High) are the raw row's
-- own, decoded:
--
--   priority 1, 2        -> CRITICAL, HIGH
--   state    3, 6, 7, 8  -> ON_HOLD, RESOLVED, CLOSED, CANCELED
--   category service_interruption -> SERVICE_INTERRUPTION
--
-- The engine's read-time translation of raw codes leaves decoded text
-- unchanged, so the row evaluates the same whether or not it has been
-- rewritten; what the rewrite changes is the service gate, and what it
-- gives the row is the same readable form the case policies have.
--
-- Backup of the row, pre-change, kept in sla_policy_backup_0293, never
-- dropped. Safe to re-run: the backup is created once and the UPDATE sets
-- the same text again.

CREATE TABLE IF NOT EXISTS sla_policy_backup_0293 AS
SELECT id, name, start_condition, stop_condition, cancel_condition
FROM sla_policy
WHERE name = 'Cloud Incident - High Priority';

UPDATE sla_policy
SET start_condition  = 'priorityINCRITICAL,HIGH^category=SERVICE_INTERRUPTION',
    stop_condition   = 'stateINON_HOLD,RESOLVED,CLOSED',
    cancel_condition = 'state=CANCELED^NQcategory!=SERVICE_INTERRUPTION^NQpriorityNOT INCRITICAL,HIGH'
WHERE name = 'Cloud Incident - High Priority';
