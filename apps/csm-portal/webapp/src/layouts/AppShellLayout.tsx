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

import { Box } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";

export interface AppShellLayoutProps {
  header: ReactNode;
  sidebar?: ReactNode;
  children: ReactNode;
}

/**
 * Application shell with a flex layout that constrains main content to the
 * viewport width remaining after the sidebar (Oxygen AppShell omits
 * minWidth: 0 on the main column, which prevents inner content from sizing
 * to the screen and silently clips it on any viewport narrower than the
 * content's intrinsic width). Ported from the customer portal's
 * `AppShellLayout` (`apps/customer-portal/webapp/src/layouts/AppShellLayout.tsx`),
 * trimmed to what CSM actually uses today: no footer slot and no
 * overlay/mobile-drawer sidebar mode (CSM's sidebar only ever renders
 * inline, collapsed/expanded via `CsmSideBar`'s own `collapsed` prop).
 *
 * @param {AppShellLayoutProps} props - Shell regions and page content.
 * @returns {JSX.Element} The app shell layout.
 */
export default function AppShellLayout({
  header,
  sidebar,
  children,
}: AppShellLayoutProps): JSX.Element {
  return (
    <Box
      data-testid="app-shell"
      className="csm-print-expand"
      sx={{
        display: "flex",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
        minWidth: 0,
        width: "100%",
        maxWidth: "100%",
        overflow: "hidden",
      }}
    >
      <Box
        component="header"
        data-testid="app-navbar"
        className="csm-print-hide"
        sx={{
          flexShrink: 0,
          width: "100%",
          maxWidth: "100%",
          minWidth: 0,
        }}
      >
        {header}
      </Box>

      <Box
        className="csm-print-expand"
        sx={{
          display: "flex",
          flex: 1,
          minHeight: 0,
          minWidth: 0,
          width: "100%",
          maxWidth: "100%",
          overflow: "hidden",
          position: "relative",
        }}
      >
        {sidebar ? (
          <Box
            component="aside"
            data-testid="app-sidebar"
            className="csm-print-hide"
            sx={{ flexShrink: 0, minWidth: 0 }}
          >
            {sidebar}
          </Box>
        ) : null}

        <Box
          component="main"
          data-testid="app-main"
          className="csm-print-expand"
          sx={{
            display: "flex",
            flexDirection: "column",
            flex: "1 1 0",
            minWidth: 0,
            width: 0,
            maxWidth: "100%",
            overflow: "hidden",
          }}
        >
          {children}
        </Box>
      </Box>
    </Box>
  );
}
