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
import { type JSX, type ReactNode, useEffect } from "react";
import { SIDEBAR_DRAWER_WIDTH_PX } from "@constants/common";

export interface AppShellLayoutProps {
  header: ReactNode;
  sidebar?: ReactNode;
  footer: ReactNode;
  children: ReactNode;
  sidebarOverlay?: boolean;
  sidebarOpen?: boolean;
  onSidebarClose?: () => void;
}

/**
 * Application shell with a flex layout that constrains main content to the
 * viewport width remaining after the sidebar (Oxygen AppShell omits minWidth: 0
 * on the main column, which prevents inner content from sizing to the screen).
 *
 * @param {AppShellLayoutProps} props - Shell regions and page content.
 * @returns {JSX.Element} The app shell layout.
 */
export default function AppShellLayout({
  header,
  sidebar,
  footer,
  children,
  sidebarOverlay = false,
  sidebarOpen = false,
  onSidebarClose,
}: AppShellLayoutProps): JSX.Element {
  useEffect(() => {
    if (!sidebarOverlay || !sidebarOpen) return;

    const handleKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") {
        onSidebarClose?.();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [sidebarOverlay, sidebarOpen, onSidebarClose]);

  return (
    <Box
      data-testid="app-shell"
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
        {sidebar && sidebarOverlay ? (
          <>
            {sidebarOpen ? (
              <Box
                aria-hidden
                data-testid="app-sidebar-backdrop"
                onClick={onSidebarClose}
                onKeyDown={() => undefined}
                sx={{
                  position: "absolute",
                  inset: 0,
                  zIndex: (theme) => theme.zIndex.drawer,
                  bgcolor: (theme) => theme.palette.action.disabledBackground,
                }}
              />
            ) : null}
            <Box
              component="aside"
              data-testid="app-sidebar-drawer"
              aria-hidden={!sidebarOpen}
              sx={{
                position: "absolute",
                left: 0,
                top: 0,
                bottom: 0,
                width: SIDEBAR_DRAWER_WIDTH_PX,
                zIndex: (theme) => theme.zIndex.drawer + 1,
                transform: sidebarOpen ? "translateX(0)" : "translateX(-100%)",
                transition: (theme) =>
                  theme.transitions.create("transform", {
                    easing: theme.transitions.easing.sharp,
                    duration: sidebarOpen
                      ? theme.transitions.duration.enteringScreen
                      : theme.transitions.duration.leavingScreen,
                  }),
                bgcolor: "background.paper",
                borderRight: 1,
                borderColor: "divider",
                boxShadow: sidebarOpen ? 8 : "none",
                pointerEvents: sidebarOpen ? "auto" : "none",
                overflow: "hidden",
              }}
            >
              {sidebar}
            </Box>
          </>
        ) : null}

        {!sidebarOverlay && sidebar ? (
          <Box
            component="aside"
            data-testid="app-sidebar"
            sx={{ flexShrink: 0, minWidth: 0 }}
          >
            {sidebar}
          </Box>
        ) : null}

        <Box
          component="main"
          data-testid="app-main"
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
          <Box
            id="main-scroll-container"
            sx={{
              display: "flex",
              flexDirection: "column",
              flex: 1,
              minHeight: 0,
              minWidth: 0,
              width: "100%",
              maxWidth: "100%",
              overflow: "auto",
            }}
          >
            {children}
          </Box>

          <Box
            component="footer"
            data-testid="app-footer"
            sx={{
              flexShrink: 0,
              width: "100%",
              maxWidth: "100%",
              minWidth: 0,
            }}
          >
            {footer}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
