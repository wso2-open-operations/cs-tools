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

-- features: one nav-node id and its shared state, edited from CSM Settings.
-- state is enabled or preview. A row is never deleted: setting a page back to
-- available updates state to enabled, the same way an unpublished banner stays
-- in portal_banner. A page with no row is available, or follows its section.
-- The menu rules stay in the webapp.
--
-- feature_saved: at most one row. Until it exists, an empty features table
-- means the portal still reads config.js. The first successful save writes
-- that mark. Entity-service owns both tables. No row-level security. The
-- admin check lives in the CSM portal backend.
--
-- A database that already has the earlier names (feature_override,
-- feature_override_mark) is renamed in place so this file stays safe to run
-- again by hand.

BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$
BEGIN
    IF to_regclass('public.feature_override') IS NOT NULL
       AND to_regclass('public.features') IS NULL THEN
        ALTER TABLE feature_override RENAME TO features;
    END IF;
    IF to_regclass('public.feature_override_mark') IS NOT NULL
       AND to_regclass('public.feature_saved') IS NULL THEN
        ALTER TABLE feature_override_mark RENAME TO feature_saved;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS features (
    nav_node_id  TEXT        PRIMARY KEY CHECK (btrim(nav_node_id) <> ''),
    state        TEXT        NOT NULL CHECK (state IN ('enabled', 'preview')),
    updated_on   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by   TEXT        NOT NULL
);

COMMENT ON TABLE features IS
    'Shared CSM page visibility. enabled or preview. Rows are never deleted. A missing row means the page was never saved.';

CREATE TABLE IF NOT EXISTS feature_saved (
    only_row  BOOLEAN     PRIMARY KEY DEFAULT TRUE CHECK (only_row),
    saved_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    saved_by  TEXT        NOT NULL
);

COMMENT ON TABLE feature_saved IS
    'One row after the first successful feature-visibility save. Until then the portal still reads config.js.';

COMMIT;
