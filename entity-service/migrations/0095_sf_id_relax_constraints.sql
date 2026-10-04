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

-- sf_id (Salesforce id) is neither unique nor always populated in production
-- Salesforce data - both NOT NULL and UNIQUE reject real rows, so every sf_id
-- column across the schema is relaxed to a plain nullable, non-unique column.
-- account/project had NOT NULL UNIQUE inline (default constraint name
-- <table>_sf_id_key); account_contact/project_contact/"user" were already
-- nullable but still UNIQUE.
ALTER TABLE account ALTER COLUMN sf_id DROP NOT NULL;
ALTER TABLE account DROP CONSTRAINT IF EXISTS account_sf_id_key;

ALTER TABLE project ALTER COLUMN sf_id DROP NOT NULL;
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_sf_id_key;

ALTER TABLE account_contact DROP CONSTRAINT IF EXISTS account_contact_sf_id_key;
ALTER TABLE project_contact DROP CONSTRAINT IF EXISTS project_contact_sf_id_key;
ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_sf_id_key;
