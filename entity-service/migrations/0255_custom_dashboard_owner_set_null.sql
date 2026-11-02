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

-- custom_dashboard (0251) kept its rows only while the owner existed:
-- owner_user_id was ON DELETE CASCADE, so deleting a user also took the
-- dashboards they had shared with people and teams. owner_user_id is now
-- nullable and ON DELETE SET NULL: a deleted user's shared dashboards stay,
-- with no owner, and nobody can edit or delete them any more. The ones shared
-- with nobody (no row in custom_dashboard_share or
-- custom_dashboard_team_share) are of no use to anyone, so the trigger below
-- deletes them as the owner goes.
--
-- Safe to re-run: dropping NOT NULL twice is a no-op, the foreign key is only
-- replaced while it is not already ON DELETE SET NULL, and the function and
-- trigger are replaced.
ALTER TABLE custom_dashboard ALTER COLUMN owner_user_id DROP NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'custom_dashboard'::regclass
           AND conname = 'custom_dashboard_owner_user_id_fkey'
           AND confdeltype = 'n'
    ) THEN
        ALTER TABLE custom_dashboard DROP CONSTRAINT IF EXISTS custom_dashboard_owner_user_id_fkey;
        ALTER TABLE custom_dashboard
            ADD CONSTRAINT custom_dashboard_owner_user_id_fkey
            FOREIGN KEY (owner_user_id) REFERENCES "user"(id) ON DELETE SET NULL;
    END IF;
END $$;

-- Fires on the UPDATE that ON DELETE SET NULL makes, once per dashboard of
-- the deleted owner.
CREATE OR REPLACE FUNCTION custom_dashboard_delete_unshared_orphan() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM custom_dashboard_share WHERE dashboard_id = NEW.id)
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_team_share WHERE dashboard_id = NEW.id) THEN
        DELETE FROM custom_dashboard WHERE id = NEW.id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_custom_dashboard_delete_unshared_orphan ON custom_dashboard;
CREATE TRIGGER trg_custom_dashboard_delete_unshared_orphan
    AFTER UPDATE OF owner_user_id ON custom_dashboard
    FOR EACH ROW
    WHEN (OLD.owner_user_id IS NOT NULL AND NEW.owner_user_id IS NULL)
    EXECUTE FUNCTION custom_dashboard_delete_unshared_orphan();
