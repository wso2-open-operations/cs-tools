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

-- A durable per-project delivery record for an announcement request's own
-- Publish fan-out (Phase 3's "batch record and delivery" backlog item,
-- scoped down to just the ledger -- no batch entity, no scheduling, no
-- expiry). Before this table, the webapp's usePublishAnnouncementRequest
-- hook tracked which projects had already received a case entirely in
-- local React state: closing the dialog mid-retry lost that progress, and
-- reopening it resent a real case to every resolved project again,
-- including ones that already succeeded -- a documented, accepted
-- limitation this table exists to close.
--
-- No row is ever written for a project that hasn't been attempted yet --
-- "pending" is never a stored status, it's just "no row exists yet for
-- this (request, project) pair" from the caller's own perspective (see
-- resolved_project_ids on announcement_requests for the full audience).
--
-- tag_failed (not just succeeded/failed) exists because a security
-- announcement's case-create and its mandatory security-tag attach are two
-- separate calls that can fail independently -- a case that exists but is
-- missing its tag must not be treated as "not sent yet" (that would create
-- a duplicate case on retry) or as "fully sent" (the tag is mandatory).
-- case_id is set for both succeeded and tag_failed, since the case is real
-- either way; only failed has no case_id.
CREATE TABLE IF NOT EXISTS announcement_request_deliveries (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  announcement_request_id UUID NOT NULL REFERENCES announcement_requests(id) ON DELETE CASCADE,
  project_id              UUID NOT NULL,
  case_id                 TEXT,
  status                  TEXT NOT NULL CHECK (status IN ('succeeded', 'tag_failed', 'failed')),
  -- Mirrors the service-layer check in RecordDeliveries: succeeded/tag_failed
  -- always have a real case, failed never does.
  CHECK (
    (status IN ('succeeded', 'tag_failed') AND case_id IS NOT NULL)
    OR (status = 'failed' AND case_id IS NULL)
  ),
  error_message           TEXT,
  created_on              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_on              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- One row per (request, project): a retry re-attempting a previously
  -- failed project must overwrite that project's own row, never insert a
  -- second one, or a caller reading this back would double-count it.
  UNIQUE (announcement_request_id, project_id)
);

CREATE INDEX IF NOT EXISTS idx_announcement_request_deliveries_request_id
  ON announcement_request_deliveries (announcement_request_id);
