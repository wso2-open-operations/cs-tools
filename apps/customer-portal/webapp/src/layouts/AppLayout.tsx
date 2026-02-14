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

import { notificationBannerConfig } from "@config/notificationBannerConfig";
import { AppShell, Box, useAppShell, LinearProgress } from "@wso2/oxygen-ui";
import { type JSX, type ReactNode, useRef, useEffect } from "react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useLocation, Outlet } from "react-router";
import GlobalNotificationBanner from "@components/common/notification-banner/GlobalNotificationBanner";
import Footer from "@components/common/footer/Footer";
import Header from "@components/common/header/Header";
import SideBar from "@components/common/side-nav-bar/SideBar";
import { BackgroundTokenRefresh } from "@providers/BackgroundTokenRefresh";

/**
 * AppLayout component.
 *
 * @returns {JSX.Element} The AppLayout component.
 */
interface AppLayoutProps {
  children?: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps): JSX.Element {
  const location = useLocation();
  const mainContentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (mainContentRef.current) {
      mainContentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  const { state: shellState, actions: shellActions } = useAppShell({
    initialCollapsed: false,
  });

  const { isVisible } = useLoader();

  const isProjectHub = location.pathname === "/";
  const isCaseDetailsPage = /\/[^/]+\/support\/cases\/[^/]+$/.test(
    location.pathname,
  );

  return (
    <>
      <BackgroundTokenRefresh />
      <GlobalNotificationBanner visible={notificationBannerConfig.visible} />
      <AppShell>
        {/* Header component. */}
        <AppShell.Navbar>
          <Header
            onToggleSidebar={shellActions.toggleSidebar}
            collapsed={shellState.sidebarCollapsed}
          />
        </AppShell.Navbar>

        {/* Side bar component. */}
        {!isProjectHub && (
          <AppShell.Sidebar>
            <SideBar
              collapsed={shellState.sidebarCollapsed}
              expandedMenus={shellState.expandedMenus}
              onSelect={shellActions.setActiveMenuItem}
              onToggleExpand={shellActions.toggleMenu}
            />
          </AppShell.Sidebar>
        )}

        {/* Main content. */}
        <AppShell.Main>
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              height: "100%",
              width: "100%",
              flex: 1,
              minHeight: 0,
              overflow: "hidden",
              position: "relative",
            }}
          >
            {isVisible && (
              <LinearProgress
                color="warning"
                sx={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  right: 0,
                  zIndex: 1300,
                  height: 3,
                }}
              />
            )}
            <Box
              ref={mainContentRef}
              sx={{
                flex: 1,
                minHeight: isCaseDetailsPage ? "60vh" : 0,
                overflow: "auto",
                display: isCaseDetailsPage ? "flex" : "block",
                flexDirection: isCaseDetailsPage ? "column" : undefined,
                ...(isCaseDetailsPage
                  ? { px: 0, pb: 0, pt: 0 }
                  : { p: 3 }),
              }}
            >
              {children || (
                <Outlet
                  context={{ sidebarCollapsed: shellState.sidebarCollapsed }}
                />
              )}
            </Box>
          </Box>
        </AppShell.Main>

        {/* Footer component. */}
        <AppShell.Footer>
          <Footer />
        </AppShell.Footer>
      </AppShell>
    </>
  );
}
