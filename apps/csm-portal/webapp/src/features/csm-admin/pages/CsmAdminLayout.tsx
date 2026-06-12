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

import { Box, Chip, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { type JSX, Suspense } from "react";
import { Outlet, useLocation, useNavigate } from "react-router";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";

interface AdminTab {
  id: string;
  label: string;
  path: string;
  wip?: boolean;
}

const ADMIN_TABS: AdminTab[] = [
  { id: "users", label: "Users", path: "/admin/users" },
  { id: "roles", label: "Roles", path: "/admin/roles", wip: true },
  { id: "groups", label: "Groups", path: "/admin/groups", wip: true },
  { id: "permissions", label: "Permissions", path: "/admin/permissions", wip: true },
];

function pickActiveTab(pathname: string): string {
  for (const t of ADMIN_TABS) {
    if (pathname === t.path || pathname.startsWith(`${t.path}/`)) {
      return t.id;
    }
  }
  return ADMIN_TABS[0].id;
}

export default function CsmAdminLayout(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const active = pickActiveTab(location.pathname);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Administration</Typography>

      <Tabs
        value={active}
        onChange={(_, id) => {
          const next = ADMIN_TABS.find((t) => t.id === id);
          if (next) void navigate(next.path);
        }}
        variant="scrollable"
        scrollButtons="auto"
      >
        {ADMIN_TABS.map((t) => (
          <Tab
            key={t.id}
            value={t.id}
            label={
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                {t.label}
                {t.wip && (
                  <Chip size="small" label="WIP" color="warning" variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                )}
              </Box>
            }
          />
        ))}
      </Tabs>

      <Suspense fallback={<RouteSuspenseFallback />}>
        <Outlet />
      </Suspense>
    </Box>
  );
}
