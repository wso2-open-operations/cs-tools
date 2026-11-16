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

-- code_dashboard: advanced dashboards a CSM portal user generated as code and
-- saved. source is the dashboard's JavaScript (its render(portal, root)
-- function), stored as opaque text: the CSM backend checks it before saving,
-- the portal checks it again before every run, and this service never parses
-- or runs it. It is a table of its own, not rows of custom_dashboard, because
-- that table's definition is the JSON format the backend validates. id is the
-- draft's own id, so saving the same draft again replaces the row. The source
-- cap fits the backend's 60000 characters of any UTF-8; a source of only
-- whitespace (newlines included, which btrim keeps) is refused.
--
-- owner_user_id is "user".id and, as custom_dashboard since 0255, nullable
-- and ON DELETE SET NULL, so that a dashboard shared with others can outlive
-- its owner once sharing exists. A dashboard shared with nobody is of no use
-- to anyone without its owner, so the trigger below deletes it as the owner
-- goes. Nothing can share a code dashboard yet, so today every one goes with
-- its owner; 0296, which adds sharing, replaces
-- code_dashboard_delete_unshared_orphan to keep the shared ones.
--
-- Postgres-only: no other data source holds it. Safe to re-run: the table,
-- index, function and trigger are created only while missing or replaced.
CREATE TABLE IF NOT EXISTS code_dashboard (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID         REFERENCES "user"(id) ON DELETE SET NULL,
    name          VARCHAR(255) NOT NULL CHECK (btrim(name) <> ''),
    source        TEXT         NOT NULL CHECK (source ~ '[^[:space:]]' AND octet_length(source) <= 262144),
    created_on    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One name per owner, ignoring case; also serves the owner's list. Ownerless
-- rows never collide: NULLs are distinct in a unique index.
CREATE UNIQUE INDEX IF NOT EXISTS code_dashboard_owner_name
    ON code_dashboard (owner_user_id, LOWER(name));

-- Fires on the UPDATE that ON DELETE SET NULL makes, once per dashboard of
-- the deleted owner. No share table exists yet, so no dashboard is kept.
CREATE OR REPLACE FUNCTION code_dashboard_delete_unshared_orphan() RETURNS trigger AS $$
BEGIN
    DELETE FROM code_dashboard WHERE id = NEW.id;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_code_dashboard_delete_unshared_orphan ON code_dashboard;
CREATE TRIGGER trg_code_dashboard_delete_unshared_orphan
    AFTER UPDATE OF owner_user_id ON code_dashboard
    FOR EACH ROW
    WHEN (OLD.owner_user_id IS NOT NULL AND NEW.owner_user_id IS NULL)
    EXECUTE FUNCTION code_dashboard_delete_unshared_orphan();
