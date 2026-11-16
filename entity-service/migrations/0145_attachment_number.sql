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

-- A short readable number for every attachment (WIA0000001, CA0000001,
-- DA0000001), used as the prefix of its file name in SFTPGo so two files with
-- the same name never share a path. Assigned by the database when a row is
-- inserted; the attachment mappings do not list it, so a re-sync keeps it.

CREATE OR REPLACE FUNCTION attachment_number(prefix text, n bigint) RETURNS text
    LANGUAGE sql IMMUTABLE AS
$$ SELECT prefix || CASE WHEN n < 10000000 THEN lpad(n::text, 7, '0') ELSE n::text END $$;

CREATE SEQUENCE IF NOT EXISTS work_item_attachment_number_seq;
CREATE SEQUENCE IF NOT EXISTS comment_attachment_number_seq;
CREATE SEQUENCE IF NOT EXISTS deployment_attachment_number_seq;

-- Add the column and its default first so rows inserted while this runs get a
-- number, then number the existing rows.
ALTER TABLE work_item_attachment ADD COLUMN IF NOT EXISTS number VARCHAR(100);
ALTER TABLE work_item_attachment ALTER COLUMN number SET DEFAULT attachment_number('WIA', nextval('work_item_attachment_number_seq'));
UPDATE work_item_attachment SET number = attachment_number('WIA', nextval('work_item_attachment_number_seq')) WHERE number IS NULL;
ALTER TABLE work_item_attachment ALTER COLUMN number SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_work_item_attachment_number ON work_item_attachment (number);

ALTER TABLE comment_attachment ADD COLUMN IF NOT EXISTS number VARCHAR(100);
ALTER TABLE comment_attachment ALTER COLUMN number SET DEFAULT attachment_number('CA', nextval('comment_attachment_number_seq'));
UPDATE comment_attachment SET number = attachment_number('CA', nextval('comment_attachment_number_seq')) WHERE number IS NULL;
ALTER TABLE comment_attachment ALTER COLUMN number SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_comment_attachment_number ON comment_attachment (number);

ALTER TABLE deployment_attachment ADD COLUMN IF NOT EXISTS number VARCHAR(100);
ALTER TABLE deployment_attachment ALTER COLUMN number SET DEFAULT attachment_number('DA', nextval('deployment_attachment_number_seq'));
UPDATE deployment_attachment SET number = attachment_number('DA', nextval('deployment_attachment_number_seq')) WHERE number IS NULL;
ALTER TABLE deployment_attachment ALTER COLUMN number SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_deployment_attachment_number ON deployment_attachment (number);
