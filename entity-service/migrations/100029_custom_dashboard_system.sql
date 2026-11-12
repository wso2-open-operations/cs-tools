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

-- System dashboards: dashboards the CSM backend ships with the product (they
-- used to be loaded from a mounted directory) kept in custom_dashboard next to
-- the ones users save, so they share its columns and its definition format.
-- A system dashboard belongs to nobody: is_system is true, owner_user_id is
-- NULL and system_key is its stable key (e.g. the file name the backend
-- derived it from). The CSM backend upserts each one by key on startup, which
-- overwrites name and definition. No user can reach one: every user-facing
-- query filters by owner_user_id or by a share row, and a system row has
-- neither.
--
-- owner_user_id has been nullable since 0255 (a deleted owner's shared
-- dashboards stay, ownerless), so the check below tells the two apart by
-- is_system, not by the owner: a system row has a key and no owner, any other
-- row has no key. The ownerless rows 0255 leaves behind stay valid.
--
-- The (owner_user_id, lower(name)) unique index (0251) is untouched: NULLs
-- never collide in a unique index, so system names need not differ across
-- keys. A system key is unique through its own partial index.
--
-- Definition size: 0251 caps a definition at 65536 octets, right for what a
-- user saves. A system dashboard can carry expanded sections and be larger, so
-- the cap becomes 1048576 octets for is_system rows only; other rows keep
-- 65536.
--
-- The orphan cleanup of 0255, 0276 and 0277 (owner deleted, last share
-- deleted) must never touch a system row. 0255's trigger already cannot fire
-- for one (it needs an owner going from set to NULL) and 0277's link-off
-- trigger needs a link that was on. custom_dashboard_delete_if_orphaned (0277)
-- would delete any ownerless row with no share left, a system row included, if
-- it were ever called with its id (a share row deleted by hand), so it and the
-- 0276 function now skip system rows. A system row also cannot have its link
-- on.
--
-- Safe to re-run: columns and indexes use IF NOT EXISTS, constraints are only
-- added while missing, and the functions are replaced.
ALTER TABLE custom_dashboard
    ADD COLUMN IF NOT EXISTS is_system  BOOLEAN      NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS system_key VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS custom_dashboard_system_key
    ON custom_dashboard (system_key) WHERE system_key IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'custom_dashboard_system_shape'
           AND conrelid = to_regclass('custom_dashboard')
    ) THEN
        ALTER TABLE custom_dashboard
            ADD CONSTRAINT custom_dashboard_system_shape
            CHECK (
                (is_system AND system_key IS NOT NULL AND btrim(system_key) <> ''
                    AND owner_user_id IS NULL AND NOT link_enabled)
                OR (NOT is_system AND system_key IS NULL)
            );
    END IF;

    -- Replace 0251's inline definition-size check (auto-named
    -- custom_dashboard_definition_check) with one that allows more for system
    -- rows. Once replaced, the new one is left alone.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'custom_dashboard_definition_size'
           AND conrelid = to_regclass('custom_dashboard')
    ) THEN
        ALTER TABLE custom_dashboard DROP CONSTRAINT IF EXISTS custom_dashboard_definition_check;
        ALTER TABLE custom_dashboard
            ADD CONSTRAINT custom_dashboard_definition_size
            CHECK (btrim(definition) <> ''
                   AND octet_length(definition) <= CASE WHEN is_system THEN 1048576 ELSE 65536 END);
    END IF;
END $$;

-- 0277's function, with system rows excluded.
CREATE OR REPLACE FUNCTION custom_dashboard_delete_if_orphaned(p_dashboard_id UUID) RETURNS void AS $$
BEGIN
    DELETE FROM custom_dashboard d
     WHERE d.id = p_dashboard_id
       AND NOT d.is_system
       AND d.owner_user_id IS NULL
       AND NOT d.link_enabled
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_share s WHERE s.dashboard_id = d.id)
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_team_share ts WHERE ts.dashboard_id = d.id);
END;
$$ LANGUAGE plpgsql;

-- 0276's function, with system rows excluded.
CREATE OR REPLACE FUNCTION custom_dashboard_delete_unshared_orphan() RETURNS trigger AS $$
BEGIN
    IF NEW.is_system THEN
        RETURN NULL;
    END IF;
    IF NEW.link_enabled THEN
        DELETE FROM custom_dashboard_link_access WHERE dashboard_id = NEW.id;
        UPDATE custom_dashboard SET link_enabled = FALSE, link_id = NULL WHERE id = NEW.id;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM custom_dashboard_share WHERE dashboard_id = NEW.id)
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_team_share WHERE dashboard_id = NEW.id) THEN
        DELETE FROM custom_dashboard WHERE id = NEW.id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
