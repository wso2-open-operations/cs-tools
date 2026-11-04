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

-- Sharing a saved custom dashboard (migration 0251) with anyone who has its
-- link: the owner turns "Anyone with the link can view" on and copies the
-- link; any internal user who opens it can then view the dashboard (CSM
-- portal Shared Dashboards) for as long as the option stays on.
--
-- custom_dashboard.link_enabled is the option. link_id is the id in the
-- link (not the dashboard's own id, so a link cannot be guessed from it): a
-- new random one each time the option is turned on, so turning it off and on
-- again makes earlier copies of the link stop working. The check keeps the
-- two in step: an id exactly when the option is on. Postgres-only: no other
-- data source holds it.
ALTER TABLE custom_dashboard
    ADD COLUMN IF NOT EXISTS link_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS link_id      UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'custom_dashboard_link_enabled_has_id'
           AND conrelid = to_regclass('custom_dashboard')
    ) THEN
        ALTER TABLE custom_dashboard
            ADD CONSTRAINT custom_dashboard_link_enabled_has_id
            CHECK (link_enabled = (link_id IS NOT NULL));
    END IF;
END $$;

-- Opening a link looks the dashboard up by its link id.
CREATE UNIQUE INDEX IF NOT EXISTS custom_dashboard_link_id
    ON custom_dashboard (link_id) WHERE link_id IS NOT NULL;

-- custom_dashboard_link_access: a user who opened a dashboard's link, and so
-- sees it under Shared Dashboards. One row per (dashboard, user); opening it
-- again changes nothing. Turning the option off deletes the dashboard's rows
-- (in the same transaction as the update), so everyone who had it only
-- through the link loses it. Only internal staff, never the owner; the
-- service checks that. Rows go with the dashboard and with the user.
CREATE TABLE IF NOT EXISTS custom_dashboard_link_access (
    dashboard_id UUID        NOT NULL REFERENCES custom_dashboard(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dashboard_id, user_id)
);

-- A user's Shared Dashboards list reads their rows; the primary key serves the
-- owner's count per dashboard.
CREATE INDEX IF NOT EXISTS custom_dashboard_link_access_user
    ON custom_dashboard_link_access (user_id);
