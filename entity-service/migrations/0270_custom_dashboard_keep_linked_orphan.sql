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
-- whose link is on (migration 0269) is shared too: with anyone who has the
-- link, opened or not yet. So it is kept as well, and its link keeps working;
-- with no owner nobody can turn it off, edit it or delete it.
--
-- Only the function changes; the trigger and when it fires are 0255's. A
-- dashboard whose link is off is decided exactly as before.
--
-- Safe to re-run: the function is replaced.
CREATE OR REPLACE FUNCTION custom_dashboard_delete_unshared_orphan() RETURNS trigger AS $$
BEGIN
    IF NOT NEW.link_enabled
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_share WHERE dashboard_id = NEW.id)
       AND NOT EXISTS (SELECT 1 FROM custom_dashboard_team_share WHERE dashboard_id = NEW.id) THEN
        DELETE FROM custom_dashboard WHERE id = NEW.id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
