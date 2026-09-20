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

DO $$ BEGIN
    CREATE TYPE user_type_enum AS ENUM ('SYSTEM', 'INTERNAL', 'EXTERNAL', 'NOT_AVAILABLE');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS user_type user_type_enum;

CREATE OR REPLACE FUNCTION recompute_user_type(p_user_id UUID) RETURNS void AS $$
BEGIN
    UPDATE "user"
    SET user_type = (CASE
        WHEN is_system_user THEN 'SYSTEM'
        WHEN EXISTS (
            SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
            WHERE ur.user_id = p_user_id AND r.name IN ('admin', 'internal')
        ) THEN 'INTERNAL'
        WHEN EXISTS (
            SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
            WHERE ur.user_id = p_user_id
              AND r.name IN ('external', 'partner', 'customer', 'partner_admin', 'customer_admin')
        ) THEN 'EXTERNAL'
        ELSE 'NOT_AVAILABLE'
    END)::user_type_enum
    WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION trg_users_recompute_type() RETURNS trigger AS $$
BEGIN
    PERFORM recompute_user_type(NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_is_system_user_change ON "user";
CREATE TRIGGER users_is_system_user_change
AFTER INSERT OR UPDATE OF is_system_user ON "user"
FOR EACH ROW EXECUTE FUNCTION trg_users_recompute_type();

-- BUGFIX: an UPDATE that reassigns a role row from one user to another (changing
-- user_role.user_id) must recompute BOTH the old and new user's type, not just
-- NEW's -- otherwise the user who lost the role keeps a stale user_type.
CREATE OR REPLACE FUNCTION trg_user_roles_recompute_type() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM recompute_user_type(OLD.user_id);
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        PERFORM recompute_user_type(NEW.user_id);
        IF OLD.user_id IS DISTINCT FROM NEW.user_id THEN
            PERFORM recompute_user_type(OLD.user_id);
        END IF;
        RETURN NEW;
    ELSE
        PERFORM recompute_user_type(NEW.user_id);
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS user_roles_change_recompute_type ON user_role;
CREATE TRIGGER user_roles_change_recompute_type
AFTER INSERT OR UPDATE OR DELETE ON user_role
FOR EACH ROW EXECUTE FUNCTION trg_user_roles_recompute_type();

-- BUGFIX: a role rename (role.name) changes which users derive to INTERNAL/
-- EXTERNAL/NOT_AVAILABLE, since recompute_user_type() matches by name, not
-- role id. Without this, renaming e.g. 'internal' to something else leaves
-- every affected user's user_type stale until their own row is next touched.
CREATE OR REPLACE FUNCTION trg_role_name_recompute_type() RETURNS trigger AS $$
BEGIN
    PERFORM recompute_user_type(ur.user_id)
    FROM user_role ur
    WHERE ur.role_id = NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS role_name_change_recompute_type ON role;
CREATE TRIGGER role_name_change_recompute_type
AFTER UPDATE OF name ON role
FOR EACH ROW EXECUTE FUNCTION trg_role_name_recompute_type();

-- One-time backfill for rows that existed before the triggers above.
UPDATE "user" u
SET user_type = (CASE
    WHEN u.is_system_user THEN 'SYSTEM'
    WHEN EXISTS (SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id WHERE ur.user_id = u.id AND r.name IN ('admin', 'internal')) THEN 'INTERNAL'
    WHEN EXISTS (SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id WHERE ur.user_id = u.id AND r.name IN ('external', 'partner', 'customer', 'partner_admin', 'customer_admin')) THEN 'EXTERNAL'
    ELSE 'NOT_AVAILABLE'
END)::user_type_enum;
