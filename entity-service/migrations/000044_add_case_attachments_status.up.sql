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

-- Adds an upload lifecycle to case_attachments. Today, a row is only ever
-- created once the browser has already finished uploading to SFTPGo (see
-- CaseService.CreateCaseAttachment); if that final registration call never
-- lands (closed tab, crash, network blip after a successful upload), SFTPGo
-- holds an orphan file with zero record in CSM. The fix: register a 'pending'
-- row before the upload credential is even minted, then transition it to
-- 'complete' once the browser reports success. Default 'complete' keeps this
-- additive and backward-compatible with any existing rows (none in practice,
-- since this feature is unshipped, but treated as a real migration regardless).
ALTER TABLE case_attachments
  ADD COLUMN status TEXT NOT NULL DEFAULT 'complete' CHECK (status IN ('pending', 'complete'));

-- Supports a future reconciliation job that scans for pending rows whose
-- upload never completed (e.g. "status = 'pending' AND created_at < now() -
-- interval '1 hour'"). Not built in this change -- see the status column
-- comment above.
CREATE INDEX IF NOT EXISTS idx_case_attachments_status_created ON case_attachments(status, created_at);

COMMIT;
