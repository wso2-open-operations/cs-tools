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

-- Tracks where each attachment file lives in SFTPGo. A NULL storage_key means
-- the bytes are still only in ServiceNow. The attachment mappings do not list
-- these columns, so a re-sync never resets them.

DO $$ BEGIN
    CREATE TYPE attachment_status_enum AS ENUM (
        'PENDING', 'COMPLETE', 'FAILED', 'SOURCE_MISSING'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE work_item_attachment
    ADD COLUMN IF NOT EXISTS status attachment_status_enum NOT NULL DEFAULT 'PENDING',
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS stored_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS status_detail TEXT;

ALTER TABLE comment_attachment
    ADD COLUMN IF NOT EXISTS status attachment_status_enum NOT NULL DEFAULT 'PENDING',
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS stored_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS status_detail TEXT;

ALTER TABLE deployment_attachment
    ADD COLUMN IF NOT EXISTS status attachment_status_enum NOT NULL DEFAULT 'PENDING',
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS stored_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS status_detail TEXT;

COMMENT ON COLUMN work_item_attachment.state IS 'Deprecated: use status.';
COMMENT ON COLUMN comment_attachment.state IS 'Deprecated: use status.';

-- The copier's work queue: rows whose file has not been stored yet.
CREATE INDEX IF NOT EXISTS idx_work_item_attachment_pending
    ON work_item_attachment (id) WHERE storage_key IS NULL AND status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_comment_attachment_pending
    ON comment_attachment (id) WHERE storage_key IS NULL AND status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_deployment_attachment_pending
    ON deployment_attachment (id) WHERE storage_key IS NULL AND status = 'PENDING';
