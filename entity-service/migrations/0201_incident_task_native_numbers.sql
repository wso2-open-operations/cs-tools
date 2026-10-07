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

-- Incident tasks created here (the specialist handoff's runbook task, the
-- incident-report and post-resolution flows' tasks) take ServiceNow's
-- TASK numbers from migration 0180's TASK series instead of the shared
-- CS-PORTAL- series, so they read like every other incident task.
--
-- ServiceNow keeps allocating TASK numbers while it syncs (TASK0084630 on
-- staging, 2026-10-07), so the series starts at TASK1000000, far above that
-- range -- the same move native outage creation made with OUT0010000. It
-- only ever moves forward: a sequence already past this point (a cutover
-- already seeded it) is left alone. scripts/cutover/seed_native_numbering.sql
-- counts these numbers too (they match ^TASK[0-9]+$) and never moves the
-- sequence back, so the cutover still works unchanged.

SELECT setval('task_number_seq', 999999, true)
WHERE (SELECT last_value FROM task_number_seq) < 999999;
