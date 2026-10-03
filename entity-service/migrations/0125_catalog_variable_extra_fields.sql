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

ALTER TABLE catalog_variable
  ADD COLUMN IF NOT EXISTS read_only BOOLEAN,
  ADD COLUMN IF NOT EXISTS hidden BOOLEAN,
  ADD COLUMN IF NOT EXISTS reference_table VARCHAR(80),
  ADD COLUMN IF NOT EXISTS max_length INTEGER,
  ADD COLUMN IF NOT EXISTS validation_name VARCHAR(255),
  ADD COLUMN IF NOT EXISTS validation_regex VARCHAR(1024),
  ADD COLUMN IF NOT EXISTS validation_message VARCHAR(512);

CREATE TABLE IF NOT EXISTS catalog_variable_choice (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    catalog_variable_id UUID REFERENCES catalog_variable(id) ON DELETE CASCADE,
    value VARCHAR(255),
    text VARCHAR(512),
    "order" INTEGER,
    is_inactive BOOLEAN
);
CREATE INDEX IF NOT EXISTS idx_catalog_variable_choice_catalog_variable_id ON catalog_variable_choice (catalog_variable_id);
