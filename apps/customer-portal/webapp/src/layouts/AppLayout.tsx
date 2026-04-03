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
import {
  AppShell,
  Box,
  useAppShell,
  LinearProgress,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX, type ReactNode, useRef, useEffect, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useLocation, Outlet } from "react-router";
import IdleTimeoutProvider from "@providers/IdleTimeoutProvider";
import GlobalNotificationBanner from "@components/common/notification-banner/GlobalNotificationBanner";
import Footer from "@components/common/footer/Footer";
import Header from "@components/common/header/Header";
import SideBar from "@components/common/side-nav-bar/SideBar";
import NoveraFloatingChat from "@components/common/novera-floating-chat/NoveraFloatingChat";
import {
  getSidebarCollapsed,
  setSidebarCollapsed,
} from "@utils/settingsStorage";
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
  const { isLoading: isAuthLoading } = useAsgardeo();

  useEffect(() => {
    if (mainContentRef.current) {
      mainContentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  const { state: shellState, actions: shellActions } = useAppShell({
    initialCollapsed: getSidebarCollapsed(),
  });

  const { isVisible } = useLoader();
  const [loadingMessage, setLoadingMessage] = useState<
    "Authenticating…" | "Fetching user info…" | "Please wait…"
  >("Authenticating…");

  // Animate loading message in the center of the page.
  useEffect(() => {
    if (!isAuthLoading) return;

    setLoadingMessage("Authenticating…");

    const t1 = setTimeout(() => {
      setLoadingMessage("Fetching user info…");
    }, 1500);

    const t2 = setTimeout(() => {
      setLoadingMessage("Please wait…");
    }, 3000);

    return () => {
      clearTimeout(t1);
      clearTimeout(t2);
    };
  }, [isAuthLoading]);

  // Persist sidebar collapsed state to localStorage
  useEffect(() => {
    setSidebarCollapsed(shellState.sidebarCollapsed);
  }, [shellState.sidebarCollapsed]);

  const isProjectHub = location.pathname === "/" || location.pathname === "";
  const isCaseDetailsPage =
    /\/projects\/[^/]+\/support\/cases\/[^/]+$/.test(location.pathname) ||
    /\/[^/]+\/support\/cases\/[^/]+$/.test(location.pathname);
  const isServiceRequestDetailsPage =
    /\/projects\/[^/]+\/(?:support|operations)\/service-requests\/[^/]+$/.test(
      location.pathname,
    ) || /\/[^/]+\/(?:support|operations)\/service-requests\/[^/]+$/.test(
      location.pathname,
    );
  const isEngagementDetailsPage = location.pathname.includes("/engagements/");
  const isSecurityReportAnalysisDetailsPage =
    /\/projects\/[^/]+\/security-center\/security-report-analysis\/[^/]+$/.test(
      location.pathname,
    ) ||
    /\/[^/]+\/security-center\/security-report-analysis\/[^/]+$/.test(
      location.pathname,
    );
  const isVulnerabilityDetailsPage =
    (/\/projects\/[^/]+\/security-center\/[^/]+$/.test(location.pathname) ||
      /\/[^/]+\/security-center\/[^/]+$/.test(location.pathname)) &&
    !location.pathname.includes("security-report-analysis");
  const isPendingUpdatesPage =
    /\/projects\/[^/]+\/updates\/pending$/.test(location.pathname) ||
    /\/[^/]+\/updates\/pending$/.test(location.pathname);
  const isUpdateLevelDetailsPage =
    /\/projects\/[^/]+\/updates\/pending\/level\/[^/]+$/.test(
      location.pathname,
    ) || /\/[^/]+\/updates\/pending\/level\/[^/]+$/.test(location.pathname);
  const isDetailsStylePage =
    isCaseDetailsPage ||
    isServiceRequestDetailsPage ||
    isEngagementDetailsPage ||
    isSecurityReportAnalysisDetailsPage ||
    isVulnerabilityDetailsPage ||
    isPendingUpdatesPage ||
    isUpdateLevelDetailsPage;

  return (
    <IdleTimeoutProvider>
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
                minHeight: 0,
                display: "flex",
                flexDirection: "column",
                overflow: "auto",
                ...(isDetailsStylePage ? { minHeight: "60vh" } : {}),
                ...(isAuthLoading
                  ? { p: 0 }
                  : isDetailsStylePage
                    ? { px: 0, pb: 0, pt: 0 }
                    : { p: 3 }),
              }}
            >
              {isAuthLoading ? (
                <Box
                  sx={{
                    flex: 1,
                    minHeight: 0,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    justifyContent: "center",
                    gap: 2,
                  }}
                >
                  <LinearProgress
                    color="warning"
                    sx={{ width: "80%", maxWidth: 400, height: 4 }}
                  />
                  <Typography variant="body2" color="text.secondary">
                    {loadingMessage}
                  </Typography>
                </Box>
              ) : (
                children || (
                  <Outlet
                    context={{ sidebarCollapsed: shellState.sidebarCollapsed }}
                  />
                )
              )}
            </Box>
            {!isProjectHub && <NoveraFloatingChat />}
          </Box>
        </AppShell.Main>

        {/* Footer component. */}
        <AppShell.Footer>
          <Footer />
        </AppShell.Footer>
      </AppShell>
    </IdleTimeoutProvider>
  );
}
