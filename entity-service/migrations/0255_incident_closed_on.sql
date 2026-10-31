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

-- Incident auto-close: when and by whom an incident was closed.
--
-- ServiceNow closed RESOLVED incidents itself, 14 days after resolved_at,
-- stamping closed_at and closed_by (= resolved_by). The CSM database is now
-- the record of truth and does that close itself (POST /incidents/autoclose,
-- driven by operations/csm-scheduled-tasks), so the two fields need columns.
-- A manual close stamps them too (updateIncidentLifecycleTx).
--
--   closed_on     when the incident entered CLOSED. NULL for incidents closed
--                 before this migration: nothing is backfilled.
--   closed_by_id  who it is attributed to; same target and ON DELETE as
--                 resolved_by_id (migration 0058).
--
-- The partial index serves the sweep's "RESOLVED and resolved long enough
-- ago" scan; it holds only RESOLVED rows, so it stays small.
--
-- Idempotent: every statement is guarded and safe to re-run.
ALTER TABLE incident ADD COLUMN IF NOT EXISTS closed_on TIMESTAMPTZ;
ALTER TABLE incident ADD COLUMN IF NOT EXISTS closed_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_incident_closed_by_id ON incident (closed_by_id);
CREATE INDEX IF NOT EXISTS idx_incident_resolved_on_resolved ON incident (resolved_on) WHERE state = 'RESOLVED';
