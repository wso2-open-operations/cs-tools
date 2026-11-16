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


-- Sharing a saved code dashboard (migration 0295) view only, the same three
-- ways and by the same rules as a custom dashboard: with one user (0252),
-- with one team (0253) and with anyone internal who has its link (0275), and
-- the same rules when its owner is deleted (0255, 0276, 0277). These are
-- tables of their own: the custom dashboard share tables reference
-- custom_dashboard, and nothing here reads or changes them.
-- Postgres-only: no other data source holds any of it. Safe to re-run: tables,
-- columns and indexes are created only while missing, the check only while
-- absent, and the functions and triggers are replaced.
--
-- code_dashboard_share: shared with one user, who can view it but not change
-- it. Only the owner adds or removes rows; the service checks that, and that
-- the user is not the owner and is internal staff. Rows go with the dashboard
-- and with the user.
CREATE TABLE IF NOT EXISTS code_dashboard_share (
    dashboard_id        UUID        NOT NULL REFERENCES code_dashboard(id) ON DELETE CASCADE,
    shared_with_user_id UUID        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dashboard_id, shared_with_user_id)
);

-- "Shared with me": a user's shares, without scanning by dashboard.
CREATE INDEX IF NOT EXISTS code_dashboard_share_user
    ON code_dashboard_share (shared_with_user_id);

-- code_dashboard_team_share: shared with one team. Membership is read live
-- from team_member, so whoever is in the team now can view it. Rows go with
-- the dashboard and with the team.
CREATE TABLE IF NOT EXISTS code_dashboard_team_share (
    dashboard_id UUID        NOT NULL REFERENCES code_dashboard(id) ON DELETE CASCADE,
    team_id      UUID        NOT NULL REFERENCES team(id) ON DELETE CASCADE,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dashboard_id, team_id)
);

CREATE INDEX IF NOT EXISTS code_dashboard_team_share_team
    ON code_dashboard_team_share (team_id);

-- The link: link_enabled is the owner's switch; link_id is the id in the link,
-- a new random one each time it is turned on (never the dashboard's own id),
-- so turning it off and on again makes earlier copies stop working. The check
-- keeps the two in step: an id exactly when it is on.
ALTER TABLE code_dashboard
    ADD COLUMN IF NOT EXISTS link_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS link_id      UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'code_dashboard_link_enabled_has_id'
           AND conrelid = to_regclass('code_dashboard')
    ) THEN
        ALTER TABLE code_dashboard
            ADD CONSTRAINT code_dashboard_link_enabled_has_id
            CHECK (link_enabled = (link_id IS NOT NULL));
    END IF;
END $$;

-- Opening a link looks the dashboard up by its link id.
CREATE UNIQUE INDEX IF NOT EXISTS code_dashboard_link_id
    ON code_dashboard (link_id) WHERE link_id IS NOT NULL;

-- code_dashboard_link_access: a user who opened the link, and so sees the
-- dashboard under Shared Dashboards. One row per (dashboard, user). Turning
-- the link off deletes the dashboard's rows in the same transaction. Only
-- internal staff, never the owner; the service checks that. Rows go with the
-- dashboard and with the user.
CREATE TABLE IF NOT EXISTS code_dashboard_link_access (
    dashboard_id UUID        NOT NULL REFERENCES code_dashboard(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dashboard_id, user_id)
);

CREATE INDEX IF NOT EXISTS code_dashboard_link_access_user
    ON code_dashboard_link_access (user_id);

-- Deleting the owner (0295's trigger, now with 0276's rule): the link is
-- turned off first, so offboarding a user leaves no working link to their
-- dashboards and the people who had one only through the link lose it. Then
-- the dashboard is deleted unless a person or a team share is left. Only the
-- function changes; the trigger and when it fires are 0295's.
CREATE OR REPLACE FUNCTION code_dashboard_delete_unshared_orphan() RETURNS trigger AS $$
BEGIN
    IF NEW.link_enabled THEN
        DELETE FROM code_dashboard_link_access WHERE dashboard_id = NEW.id;
        UPDATE code_dashboard SET link_enabled = FALSE, link_id = NULL WHERE id = NEW.id;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM code_dashboard_share WHERE dashboard_id = NEW.id)
       AND NOT EXISTS (SELECT 1 FROM code_dashboard_team_share WHERE dashboard_id = NEW.id) THEN
        DELETE FROM code_dashboard WHERE id = NEW.id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- An ownerless dashboard is kept while a person or a team still has it (0277's
-- rule): it is deleted once it has no owner, its link is off and no share is
-- left. This runs after a share row is deleted (shares go when the user or
-- the team is deleted) and after an ownerless dashboard's link is turned off.
-- A dashboard that has its owner is never touched.
CREATE OR REPLACE FUNCTION code_dashboard_delete_if_orphaned(p_dashboard_id UUID) RETURNS void AS $$
BEGIN
    DELETE FROM code_dashboard d
     WHERE d.id = p_dashboard_id
       AND d.owner_user_id IS NULL
       AND NOT d.link_enabled
       AND NOT EXISTS (SELECT 1 FROM code_dashboard_share s WHERE s.dashboard_id = d.id)
       AND NOT EXISTS (SELECT 1 FROM code_dashboard_team_share ts WHERE ts.dashboard_id = d.id);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION code_dashboard_share_deleted() RETURNS trigger AS $$
BEGIN
    PERFORM code_dashboard_delete_if_orphaned(OLD.dashboard_id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION code_dashboard_orphan_link_off() RETURNS trigger AS $$
BEGIN
    PERFORM code_dashboard_delete_if_orphaned(NEW.id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_code_dashboard_share_deleted ON code_dashboard_share;
CREATE TRIGGER trg_code_dashboard_share_deleted
    AFTER DELETE ON code_dashboard_share
    FOR EACH ROW
    EXECUTE FUNCTION code_dashboard_share_deleted();

DROP TRIGGER IF EXISTS trg_code_dashboard_team_share_deleted ON code_dashboard_team_share;
CREATE TRIGGER trg_code_dashboard_team_share_deleted
    AFTER DELETE ON code_dashboard_team_share
    FOR EACH ROW
    EXECUTE FUNCTION code_dashboard_share_deleted();

DROP TRIGGER IF EXISTS trg_code_dashboard_orphan_link_off ON code_dashboard;
CREATE TRIGGER trg_code_dashboard_orphan_link_off
    AFTER UPDATE OF link_enabled ON code_dashboard
    FOR EACH ROW
    WHEN (OLD.link_enabled AND NOT NEW.link_enabled AND NEW.owner_user_id IS NULL)
    EXECUTE FUNCTION code_dashboard_orphan_link_off();
