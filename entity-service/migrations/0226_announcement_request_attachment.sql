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

-- Files attached to an announcement request while it is still a draft, in
-- review or approved. They are held here, not in the browser and not on a case,
-- because a request outlives the session that wrote it (draft -> approval ->
-- publish can span days and people, and a scheduled publish runs on the server
-- with no browser at all), and because the real cases do not exist yet: a
-- publish creates one case per project, and each of those cases gets its own
-- copy of these files through the ordinary case attachment path (see
-- AnnouncementRequestService.AttachToCase).
--
-- The bytes live in this table rather than in SFTPGo: under
-- DATA_SOURCE=postgres-servicenow-dual-write, the data source this feature is
-- built for, SFTPGo is never used and the case's own copy of a file is held by
-- ServiceNow, so there is no object store to reference. Announcement
-- attachments are a handful of documents of at most 10 MB each (the service
-- caps both), added by staff, which is a size a bytea column holds without
-- strain. Like announcement_requests itself this table is always PostgreSQL
-- and has no row level security: only the CSM portal's announcement endpoints
-- reach it, and they are internal-only.
--
-- Deleted with the request (ON DELETE CASCADE). The rows are kept after the
-- request is published, as the record of what was sent.
CREATE TABLE IF NOT EXISTS announcement_request_attachment (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  announcement_request_id UUID NOT NULL REFERENCES announcement_requests(id) ON DELETE CASCADE,
  name                    TEXT NOT NULL,
  mime_type               TEXT NOT NULL,
  size_bytes              BIGINT NOT NULL CHECK (size_bytes > 0),
  content                 BYTEA NOT NULL,
  -- Same convention as announcement_requests / announcement_request_updates:
  -- created_by is the IdP account id, created_by_email its readable companion.
  created_by              TEXT NOT NULL,
  created_by_email        TEXT,
  created_on              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_announcement_request_attachment_request
  ON announcement_request_attachment (announcement_request_id, created_on);
