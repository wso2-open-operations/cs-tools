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

-- published_case_ids is the real case id created for each project in
-- resolved_project_ids once Publish's fan-out succeeds -- unlike
-- resolved_project_ids (frozen at Submit), this is populated incrementally
-- by the caller as each POST /cases call succeeds, then written once
-- alongside the approved -> published transition. Self-reported by the
-- same authenticated, creator-only caller MarkPublished already restricts
-- this transition to, not independently re-verified against the case
-- table -- the same trust model dry_run_case_id already established for
-- this table (see that column's own domain doc comment: "this service
-- does not create it, does not validate that it exists").
ALTER TABLE announcement_requests ADD COLUMN IF NOT EXISTS published_case_ids JSONB;

-- A published announcement can receive follow-up updates after the fact
-- (e.g. a correction or additional detail the CS/Security team asks to have
-- appended) -- see ServiceNow's own "Announcement Update with Comments"
-- flow, which this table's owning feature replaces. Append-only: there is
-- no update/delete path, matching comment/work_item_activity's own
-- audit-log shape elsewhere in this schema. No FK to case(id) for the same
-- reason announcement_requests itself has none -- the cases this fans out
-- to are ServiceNow-backed in production, with no local `cases` row.
CREATE TABLE IF NOT EXISTS announcement_request_updates (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  announcement_request_id UUID NOT NULL REFERENCES announcement_requests(id) ON DELETE CASCADE,
  content                 TEXT NOT NULL,
  created_by              TEXT NOT NULL,
  created_on              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_announcement_request_updates_request_id
  ON announcement_request_updates (announcement_request_id, created_on DESC);
