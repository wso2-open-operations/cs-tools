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

-- When and by whom an incident was closed.
--
-- Incidents are closed 14 days after they are resolved (POST /incidents/autoclose,
-- driven by operations/csm-scheduled-tasks), and by hand, so the two facts need
-- columns. Both are stamped by updateIncidentLifecycleTx on entering CLOSED.
--
--   closed_on     when the incident entered CLOSED. NULL for incidents closed
--                 before this migration: nothing is backfilled.
--   closed_by_id  who the close is attributed to: the closer on a manual close,
--                 the auto-close's system user when it has a user row, NULL
--                 otherwise. Same target and ON DELETE as resolved_by_id
--                 (migration 0058).
--
-- The two indexes are built concurrently by migrations 0274 and 0275, each in a
-- file of its own, so building them does not block incident writes.
--
-- Safe to run again: every statement is guarded. The foreign key takes a lock on
-- "user" as well as incident, so the wait for the locks is bounded, as in
-- 0249, rather than queueing every user lookup behind a long transaction.
SET lock_timeout = '5s';

ALTER TABLE incident ADD COLUMN IF NOT EXISTS closed_on TIMESTAMPTZ;
ALTER TABLE incident ADD COLUMN IF NOT EXISTS closed_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL;

RESET lock_timeout;
