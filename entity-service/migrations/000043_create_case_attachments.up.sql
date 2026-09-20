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

BEGIN;

-- case_attachments stores metadata only. File bytes for this data source live
-- externally in SFTPGo, addressed by storage_key -- there is no base64-payload
-- alternative here, unlike the ServiceNow data source's /attachments API.
CREATE TABLE IF NOT EXISTS case_attachments (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id     UUID NOT NULL REFERENCES "case"(id),
  storage_key TEXT NOT NULL,
  filename    TEXT NOT NULL,
  mime_type   TEXT NOT NULL,
  size_bytes  BIGINT NOT NULL CHECK (size_bytes > 0),
  description TEXT,
  uploaded_by UUID NOT NULL REFERENCES "user"(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by  UUID REFERENCES "user"(id)
);

-- FK / equality indexes
CREATE INDEX IF NOT EXISTS idx_case_attachments_case_id     ON case_attachments(case_id);
CREATE INDEX IF NOT EXISTS idx_case_attachments_uploaded_by ON case_attachments(uploaded_by);

-- Composite index for the paginated per-case feed (most recent first).
CREATE INDEX IF NOT EXISTS idx_case_attachments_case_created ON case_attachments(case_id, created_at DESC);

COMMIT;
