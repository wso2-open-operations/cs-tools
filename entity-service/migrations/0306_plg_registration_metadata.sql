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

-- PLG: a signup's visit and UTM context, one row per pairing. No person is
-- identified. Safe to re-run.

CREATE TABLE IF NOT EXISTS plg_registration_metadata (
    org_platform_id          UUID PRIMARY KEY REFERENCES plg_org_platform (id) ON DELETE CASCADE,
    utm_source               TEXT,
    utm_medium               TEXT,
    utm_campaign             TEXT,
    initial_utm_source       TEXT,
    initial_utm_medium       TEXT,
    initial_utm_campaign     TEXT,
    referring_domain         TEXT,
    initial_referring_domain TEXT,
    sign_in_method           TEXT,
    preferred_language       TEXT,
    created_on               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE plg_registration_metadata IS
    'Signup visit context from Moesif: utm_* of the landing URL, initial_* of the first visit.';
