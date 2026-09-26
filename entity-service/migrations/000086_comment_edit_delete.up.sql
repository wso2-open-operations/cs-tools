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

-- migration 000037's own comment argued comment rows are append-only, mirroring
-- ServiceNow's sys_journal_field -- that rationale no longer holds now that CSM
-- Portal itself is a primary writer of this table (not just a mirror of SN's
-- journal), so this migration adds edit/soft-delete support on top of it.
ALTER TABLE comment
    ADD COLUMN deleted_at TIMESTAMPTZ NULL,
    ADD COLUMN deleted_by VARCHAR(255) NULL,
    ADD COLUMN last_edited_at TIMESTAMPTZ NULL;

-- comment_edit_history keeps every prior body as its own row, for audit --
-- body is always the PRE-edit content (what it was before the edit that
-- created this row), never the post-edit one.
CREATE TABLE IF NOT EXISTS comment_edit_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    comment_id UUID NOT NULL REFERENCES comment(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    edited_by VARCHAR(255) NOT NULL,
    edited_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_comment_edit_history_comment_id_edited_at
    ON comment_edit_history (comment_id, edited_at DESC);
