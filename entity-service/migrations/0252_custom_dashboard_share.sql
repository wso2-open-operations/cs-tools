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

-- custom_dashboard_share: a saved custom dashboard (migration 0251) shared
-- with one user, who can view it (CSM portal Shared Dashboards) but not change
-- it. Only the dashboard's owner adds or removes rows; the service checks
-- that, and that the user is not the owner and is internal staff. One row per
-- (dashboard, user). Rows go with the dashboard and with the user.
-- Postgres-only: no other data source holds it. Sharing with a team is a
-- table of its own beside this one (0253).
CREATE TABLE IF NOT EXISTS custom_dashboard_share (
    dashboard_id        UUID        NOT NULL REFERENCES custom_dashboard(id) ON DELETE CASCADE,
    shared_with_user_id UUID        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dashboard_id, shared_with_user_id)
);

-- "Shared with me": a user's shares, without scanning by dashboard.
CREATE INDEX IF NOT EXISTS custom_dashboard_share_user
    ON custom_dashboard_share (shared_with_user_id);
