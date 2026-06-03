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

import { type JSX } from "react";
import { Header as HeaderUI } from "@wso2/oxygen-ui";
import { useLocation } from "react-router";
import { Box } from "@wso2/oxygen-ui";
import Brand from "@components/header/Brand";
import Actions from "@components/header/Actions";
import MockDataToggle from "@components/header/MockDataToggle";
import CsmGlobalSearch from "@features/csm-search/components/CsmGlobalSearch";
import NotificationsBell from "@features/csm-notifications/components/NotificationsBell";
import RecentViewsButton from "@features/csm-recent/components/RecentViewsButton";

interface HeaderProps {
  onToggleSidebar: () => void;
  collapsed?: boolean;
  hideProjectControls?: boolean;
}

/**
 * CSM portal top bar.
 *
 * Holds: sidebar toggle, brand, global CSM search, and the user actions
 * (theme toggle + user menu). The legacy customer-portal project switcher,
 * project-scoped SearchBar, and "Get help" dropdown were removed earlier;
 * CSM uses the global search across cases/projects/accounts instead, and
 * project selection happens via the Projects main-menu page.
 */
export default function Header({
  onToggleSidebar,
  hideProjectControls = false,
}: HeaderProps): JSX.Element {
  const location = useLocation();
  const isProjectHub = location.pathname === "/";

  return (
    <HeaderUI>
      {!isProjectHub && !hideProjectControls && (
        <HeaderUI.Toggle collapsed={false} onToggle={onToggleSidebar} />
      )}
      <Brand />
      {!hideProjectControls && <CsmGlobalSearch />}
      <HeaderUI.Spacer />
      {!hideProjectControls && (
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, mr: 1 }}>
          <MockDataToggle />
          <RecentViewsButton />
          <NotificationsBell />
        </Box>
      )}
      <Actions hideGetHelp={hideProjectControls} />
    </HeaderUI>
  );
}
