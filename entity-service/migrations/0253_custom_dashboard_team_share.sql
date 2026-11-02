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

-- custom_dashboard_team_share: a saved custom dashboard (migration 0251)
-- shared with one team (a row of the team table), the team-level
-- sibling of custom_dashboard_share (0252). Membership is read live from
-- team_member when a dashboard is listed, so whoever is in the team now can
-- view it (CSM portal Shared Dashboards) and someone who leaves the team stops
-- seeing it. Only the dashboard's owner adds or removes rows; the service
-- checks that. One row per (dashboard, team). Rows go with the dashboard and
-- with the team. Postgres-only: no other data source holds it.
CREATE TABLE IF NOT EXISTS custom_dashboard_team_share (
    dashboard_id UUID        NOT NULL REFERENCES custom_dashboard(id) ON DELETE CASCADE,
    team_id      UUID        NOT NULL REFERENCES team(id) ON DELETE CASCADE,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dashboard_id, team_id)
);

-- "Shared with me": the shares of the caller's teams, without scanning by
-- dashboard.
CREATE INDEX IF NOT EXISTS custom_dashboard_team_share_team
    ON custom_dashboard_team_share (team_id);
