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

-- custom_dashboard: dashboards a CSM portal user generated and saved (My
-- Dashboards). definition is the dashboard JSON in the portal's file format,
-- stored as opaque text like user_saved_filter.qs: the CSM backend validates
-- it before saving and again after reading, and this service does not parse
-- it (so nothing migrates a saved definition when that format changes; see
-- the CSM backend's CLAUDE.md, "Custom dashboards"). id is the draft's own
-- id, so saving the same draft again replaces the row. name is the
-- definition's displayName, kept as a column so a list needs no JSON.
-- Postgres-only: no other data source holds it. owner_user_id is "user".id
-- (JWT email -> GetUserByEmail); rows go when the user is deleted.
CREATE TABLE IF NOT EXISTS custom_dashboard (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID         NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name          VARCHAR(255) NOT NULL CHECK (btrim(name) <> ''),
    definition    TEXT         NOT NULL CHECK (btrim(definition) <> '' AND octet_length(definition) <= 65536),
    created_on    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One name per owner, ignoring case; also serves the owner's list.
CREATE UNIQUE INDEX IF NOT EXISTS custom_dashboard_owner_name
    ON custom_dashboard (owner_user_id, LOWER(name));
