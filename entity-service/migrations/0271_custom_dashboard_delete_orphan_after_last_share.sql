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

-- A dashboard whose owner was deleted is kept while somebody can still see it
-- (migrations 0255 and 0270): a person or a team it is shared with. Its link
-- is off (0270 turns it off as the owner goes). With no owner, nobody can
-- unshare it through the API; its shares go when the person or the team is
-- deleted (both cascade). Once the last of them goes, nobody can see it any
-- more, so it is deleted here, as 0255 does for one shared with nobody when
-- its owner goes.
--
-- custom_dashboard_delete_if_orphaned(id) deletes the dashboard only when it
-- has no owner, its link is off and it has no person or team share left.
-- It runs:
--   * after a person share or a team share row is deleted;
--   * after a dashboard with no owner has its link turned off: 0270 does
--     that as the owner is deleted (no API can, turning a link off needs the
--     owner).
-- A dashboard that still has its owner is never touched.
--
-- When the dashboard itself is being deleted, its share rows cascade and the
-- triggers find no row to delete. When the owner and the last recipient are
-- deleted in one statement, whichever trigger runs last sees the other change
-- and deletes the dashboard.
--
-- Safe to re-run: the functions are replaced and the triggers are dropped and
-- created again.
CREATE OR REPLACE FUNCTION custom_dashboard_delete_if_orphaned(p_dashboard_id UUID) RETURNS void AS $$
BEGIN
    DELETE FROM custom_dashboard d
     WHERE d.id = p_dashboard_id
       AND d.owner_user_id IS NULL
       AND NOT d.link_enabled
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_share s WHERE s.dashboard_id = d.id)
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_team_share ts WHERE ts.dashboard_id = d.id);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION custom_dashboard_share_deleted() RETURNS trigger AS $$
BEGIN
    PERFORM custom_dashboard_delete_if_orphaned(OLD.dashboard_id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION custom_dashboard_orphan_link_off() RETURNS trigger AS $$
BEGIN
    PERFORM custom_dashboard_delete_if_orphaned(NEW.id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_custom_dashboard_share_deleted ON custom_dashboard_share;
CREATE TRIGGER trg_custom_dashboard_share_deleted
    AFTER DELETE ON custom_dashboard_share
    FOR EACH ROW
    EXECUTE FUNCTION custom_dashboard_share_deleted();

DROP TRIGGER IF EXISTS trg_custom_dashboard_team_share_deleted ON custom_dashboard_team_share;
CREATE TRIGGER trg_custom_dashboard_team_share_deleted
    AFTER DELETE ON custom_dashboard_team_share
    FOR EACH ROW
    EXECUTE FUNCTION custom_dashboard_share_deleted();

DROP TRIGGER IF EXISTS trg_custom_dashboard_orphan_link_off ON custom_dashboard;
CREATE TRIGGER trg_custom_dashboard_orphan_link_off
    AFTER UPDATE OF link_enabled ON custom_dashboard
    FOR EACH ROW
    WHEN (OLD.link_enabled AND NOT NEW.link_enabled AND NEW.owner_user_id IS NULL)
    EXECUTE FUNCTION custom_dashboard_orphan_link_off();
