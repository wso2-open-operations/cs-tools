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

import { Box, Link, Sidebar, Typography } from "@wso2/oxygen-ui";
import {
  Briefcase,
  Building,
  ChartColumn,
  Clock,
  Cog,
  FolderOpen,
  Headset,
  RefreshCw,
  Settings,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import { type ComponentType, type JSX } from "react";
import { Link as NavigateLink, useLocation } from "react-router";

interface CsmSideBarProps {
  collapsed: boolean;
  expandedMenus?: Record<string, boolean>;
  onSelect?: (id: string) => void;
  onToggleExpand?: (id: string) => void;
}

interface CsmNavItem {
  id: string;
  label: string;
  path: string;
  icon: ComponentType<{ size?: number | string }>;
  wip?: boolean;
}

const CSM_NAV_ITEMS: CsmNavItem[] = [
  { id: "dashboard", label: "Dashboard", path: "/dashboard", icon: ChartColumn },
  { id: "cases", label: "Cases", path: "/cases", icon: Headset },
  { id: "operations", label: "Operations", path: "/operations", icon: Cog },
  { id: "engagements", label: "Engagements", path: "/engagements", icon: Briefcase },
  { id: "updates", label: "Updates", path: "/updates", icon: RefreshCw },
  { id: "security-center", label: "Security center", path: "/security-center", icon: Shield },
  { id: "time-cards", label: "Time cards", path: "/time-cards", icon: Clock },
  { id: "accounts", label: "Accounts", path: "/accounts", icon: Building },
  { id: "projects", label: "Projects", path: "/projects", icon: FolderOpen },
  { id: "admin", label: "Administration", path: "/admin", icon: Settings },
];

function pickActiveId(pathname: string): string {
  if (pathname === "/" || pathname === "") return "dashboard";
  for (const item of CSM_NAV_ITEMS) {
    if (pathname === item.path || pathname.startsWith(`${item.path}/`)) {
      return item.id;
    }
  }
  return "accounts";
}

export default function CsmSideBar({
  collapsed,
  expandedMenus,
  onSelect,
  onToggleExpand,
}: CsmSideBarProps): JSX.Element {
  const location = useLocation();
  const activeItem = pickActiveId(location.pathname);

  return (
    <Sidebar
      collapsed={collapsed}
      activeItem={activeItem}
      expandedMenus={expandedMenus}
      onSelect={onSelect}
      onToggleExpand={onToggleExpand}
    >
      <Sidebar.Nav>
        <Sidebar.Category>
          {CSM_NAV_ITEMS.map((item) => (
            <Link
              key={item.id}
              component={NavigateLink}
              to={item.path}
              color="inherit"
              underline="none"
            >
              <Sidebar.Item id={item.id}>
                <Sidebar.ItemIcon>
                  <item.icon size={20} />
                </Sidebar.ItemIcon>
                {/* `ItemLabel` children must be a plain string: oxygen-ui
                    stringifies it for the collapsed-state tooltip. Wrapping
                    in a Box yields "[object Object]" on hover. */}
                <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
                {item.wip && !collapsed && (
                  <Sidebar.ItemBadge color="warning">WIP</Sidebar.ItemBadge>
                )}
              </Sidebar.Item>
            </Link>
          ))}
        </Sidebar.Category>
      </Sidebar.Nav>
      {/* Footer (relocated from page bottom into the sidebar to free vertical space). */}
      <Box
        sx={{
          mt: "auto",
          px: 2,
          py: 1.5,
          borderTop: 1,
          borderColor: "divider",
          display: collapsed ? "none" : "flex",
          flexDirection: "column",
          gap: 0.5,
        }}
      >
        <Typography variant="caption" color="text.secondary">
          © {new Date().getFullYear()} WSO2 LLC.
        </Typography>
        <Box sx={{ display: "flex", gap: 1.5, flexWrap: "wrap" }}>
          <Link
            href="https://wso2.com/terms-of-use/"
            target="_blank"
            rel="noopener noreferrer"
            variant="caption"
            color="text.secondary"
            underline="hover"
          >
            Terms
          </Link>
          <Link
            href="https://wso2.com/privacy-policy/"
            target="_blank"
            rel="noopener noreferrer"
            variant="caption"
            color="text.secondary"
            underline="hover"
          >
            Privacy
          </Link>
        </Box>
      </Box>
    </Sidebar>
  );
}
