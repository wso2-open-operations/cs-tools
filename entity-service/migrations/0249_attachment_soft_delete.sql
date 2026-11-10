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

-- Soft delete for attachments held in SFTPGo. A person deleting an attachment
-- removes its file from SFTPGo and marks the row DELETED; the row stays, for the
-- audit trail, with who deleted it and when. Lists and download links skip it.
--
--   status      gains DELETED.
--   deleted_on  when the attachment was deleted; NULL otherwise.
--   deleted_by  who deleted it (their email, as created_by holds the uploader's).
--
-- The attachment mappings of csm-sync-service do not list these columns, so a
-- re-sync never resets them. Mirror this file verbatim into that service: it owns
-- these tables.

ALTER TYPE attachment_status_enum ADD VALUE IF NOT EXISTS 'DELETED';

ALTER TABLE work_item_attachment
    ADD COLUMN IF NOT EXISTS deleted_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(255);

ALTER TABLE comment_attachment
    ADD COLUMN IF NOT EXISTS deleted_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(255);

ALTER TABLE deployment_attachment
    ADD COLUMN IF NOT EXISTS deleted_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(255);
