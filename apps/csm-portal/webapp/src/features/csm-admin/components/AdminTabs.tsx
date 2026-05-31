// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useLocation, useNavigate } from "react-router";

type AdminTabId =
  | "users"
  | "roles"
  | "groups"
  | "teams"
  | "skills"
  | "templates"
  | "catalog"
  | "sla"
  | "reports"
  | "closure";

const TAB_TO_PATH: Record<AdminTabId, string> = {
  users: "/admin/users",
  roles: "/admin/roles",
  groups: "/admin/groups",
  teams: "/admin/teams",
  skills: "/admin/skills",
  templates: "/admin/templates",
  catalog: "/admin/catalog",
  sla: "/admin/sla",
  reports: "/admin/reports",
  closure: "/admin/closure",
};

function activeTabFor(pathname: string): AdminTabId {
  if (pathname.startsWith("/admin/roles")) return "roles";
  if (pathname.startsWith("/admin/groups")) return "groups";
  if (pathname.startsWith("/admin/teams")) return "teams";
  if (pathname.startsWith("/admin/skills")) return "skills";
  if (pathname.startsWith("/admin/templates")) return "templates";
  if (pathname.startsWith("/admin/catalog")) return "catalog";
  if (pathname.startsWith("/admin/sla")) return "sla";
  if (pathname.startsWith("/admin/reports")) return "reports";
  if (pathname.startsWith("/admin/closure")) return "closure";
  return "users";
}

/**
 * Shared page header for all `/admin/*` routes. Tabs cover the full
 * administration surface: identity (users/roles/groups), team & skill
 * structure, response templates, service catalog, SLA policy, scheduled
 * reports, and subscription closure flow.
 */
export default function AdminTabs(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const active = activeTabFor(location.pathname);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box>
        <Typography variant="h5">Administration</Typography>
        <Typography variant="body2" color="text.secondary">
          Identity, team structure, response templates, service catalog, SLAs, reports, and closures.
        </Typography>
      </Box>
      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={active}
          onChange={(_, value) => {
            const next = value as AdminTabId;
            navigate(TAB_TO_PATH[next]);
          }}
          variant="scrollable"
          scrollButtons="auto"
        >
          <Tab value="users" label="Users" />
          <Tab value="roles" label="Roles" />
          <Tab value="groups" label="Groups" />
          <Tab value="teams" label="Teams" />
          <Tab value="skills" label="Skills" />
          <Tab value="templates" label="Response templates" />
          <Tab value="catalog" label="Service catalog" />
          <Tab value="sla" label="SLA policy" />
          <Tab value="reports" label="Reports" />
          <Tab value="closure" label="Subscription closure" />
        </Tabs>
      </Box>
    </Box>
  );
}
