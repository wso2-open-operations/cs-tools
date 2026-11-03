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

-- When a user is deleted, migration 0255's trigger deletes their dashboards
-- that are shared with nobody (no row in custom_dashboard_share or
-- custom_dashboard_team_share) and keeps the rest, with no owner. A dashboard
-- whose link is on (migration 0269) has its link turned off first: with no
-- owner nobody could turn it off, so offboarding a user must not leave a link
-- to their dashboard working. The people who had it only through the link
-- lose it (their custom_dashboard_link_access rows are deleted, as when the
-- owner turns the link off); people it is shared with directly or through a
-- team keep it, as before. So a dashboard whose link was on is then kept or
-- deleted by 0255's rule, exactly like one whose link was off.
--
-- Turning the link off also fires migration 0271's trigger for an ownerless
-- dashboard, which applies the same rule; the delete below finds nothing left
-- to do in that case.
--
-- Only the function changes; the trigger and when it fires are 0255's.
--
-- Safe to re-run: the function is replaced.
CREATE OR REPLACE FUNCTION custom_dashboard_delete_unshared_orphan() RETURNS trigger AS $$
BEGIN
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
