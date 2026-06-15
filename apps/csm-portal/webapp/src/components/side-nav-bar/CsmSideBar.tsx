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

import { Link, Sidebar } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Link as NavigateLink, useLocation } from "react-router";
import { CSM_NAV_ITEMS, navItemForPath } from "@config/csmNavItems";

interface CsmSideBarProps {
  collapsed: boolean;
  expandedMenus?: Record<string, boolean>;
  onSelect?: (id: string) => void;
  onToggleExpand?: (id: string) => void;
}

function pickActiveId(pathname: string): string {
  if (pathname === "/" || pathname === "") return "accounts";
  return navItemForPath(pathname)?.id ?? "accounts";
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
                {/* Plain string: Oxygen derives the collapsed-rail tooltip via
                    String(ItemLabel.children), so a wrapper element would render
                    as "[object Object]". */}
                <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
              </Sidebar.Item>
            </Link>
          ))}
        </Sidebar.Category>
      </Sidebar.Nav>
    </Sidebar>
  );
}
