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
  Box,
  useAppShell,
  useMediaQuery,
  useTheme,
  LinearProgress,
  Typography,
} from "@wso2/oxygen-ui";
import AppShellLayout from "@layouts/AppShellLayout";
import { type JSX, type ReactNode, useRef, useEffect, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorPageContext } from "@context/error-page/ErrorPageContext";
import { useLocation, Outlet } from "react-router";
import IdleTimeoutProvider from "@providers/IdleTimeoutProvider";
import GlobalNotificationBanner from "@components/notification-banner/GlobalNotificationBanner";
import HtmlAnnouncementBanner from "@components/announcement-banner/HtmlAnnouncementBanner";
import TopBanner from "@components/top-banner/TopBanner";
import Footer from "@components/footer/Footer";
import Header from "@components/header/Header";
import SideBar from "@components/side-nav-bar/SideBar";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import PortalAccessRequiredPage from "@/components/access-control/PortalAccessRequiredPage";
import { isUnauthorizedError } from "@utils/ApiError";
import {
  getSidebarCollapsed,
  setSidebarCollapsed,
} from "@features/settings/utils/settingsStorage";
import { useIsMidSizeTouchViewport } from "@hooks/useResponsiveLayout";

/**
 * AppLayout component.
 *
 * @returns {JSX.Element} The AppLayout component.
 */
interface AppLayoutProps {
  children?: ReactNode;
}

/**
 * AppLayout component providing the main structure, navigation, and global UI elements.
 */
export default function AppLayout({ children }: AppLayoutProps): JSX.Element {
  const location = useLocation();
  const mainContentRef = useRef<HTMLDivElement>(null);
  const { isLoading: isAuthLoading } = useAsgardeo();
  const { error: userDetailsError, isLoading: isUserDetailsLoading } =
    useGetUserDetails();
  const { isErrorPageDisplayed } = useErrorPageContext();

  // Scroll to top on route change
  useEffect(() => {
    if (mainContentRef.current) {
      mainContentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  const theme = useTheme();
  const isCompactViewport = useMediaQuery(theme.breakpoints.down("md"));
  const isMidSizeTouchViewport = useIsMidSizeTouchViewport();
  const { state: shellState, actions: shellActions } = useAppShell({
    initialCollapsed:
      getSidebarCollapsed() || isCompactViewport || isMidSizeTouchViewport,
  });

  const { isVisible } = useLoader();
  const isLoginCallback =
    new URLSearchParams(location.search).has("code") &&
    new URLSearchParams(location.search).has("state");

  // Track when auth + user details have both settled for the first time.
  // Using state (not a ref) so the component re-renders once ready.
  const [hasInitialized, setHasInitialized] = useState(false);
  const [loadingMessage, setLoadingMessage] = useState<string>(
    isLoginCallback ? "Authenticating…" : "Loading…",
  );

  useEffect(() => {
    if (!isAuthLoading && !isUserDetailsLoading) {
      setHasInitialized(true);
    }
  }, [isAuthLoading, isUserDetailsLoading]);

  // Animate loading message only during the actual login callback flow.
  useEffect(() => {
    if (!isAuthLoading || !isLoginCallback) return;

    setLoadingMessage("Authenticating…");
    const t1 = setTimeout(() => setLoadingMessage("Fetching user info…"), 1500);
    const t2 = setTimeout(() => setLoadingMessage("Please wait…"), 3000);
    return () => {
      clearTimeout(t1);
      clearTimeout(t2);
    };
  }, [isAuthLoading, isLoginCallback]);

  // Persist sidebar state
  useEffect(() => {
    setSidebarCollapsed(shellState.sidebarCollapsed);
  }, [shellState.sidebarCollapsed]);

  // Collapse sidebar when the viewport shrinks below md (not on manual expand).
  const wasCompactViewport = useRef(isCompactViewport);
  useEffect(() => {
    if (
      isCompactViewport &&
      !wasCompactViewport.current &&
      !shellState.sidebarCollapsed
    ) {
      shellActions.toggleSidebar();
    }
    wasCompactViewport.current = isCompactViewport;
  }, [isCompactViewport, shellState.sidebarCollapsed, shellActions]);

  const isSidebarOverlay = isMidSizeTouchViewport;
  const isSidebarOpen = isSidebarOverlay && !shellState.sidebarCollapsed;

  // Collapse sidebar when entering overlay mode so the drawer does not open unexpectedly.
  const previousIsSidebarOverlay = useRef(isSidebarOverlay);
  useEffect(() => {
    if (
      !previousIsSidebarOverlay.current &&
      isSidebarOverlay &&
      !shellState.sidebarCollapsed
    ) {
      shellActions.toggleSidebar();
    }
    previousIsSidebarOverlay.current = isSidebarOverlay;
  }, [isSidebarOverlay, shellState.sidebarCollapsed, shellActions]);

  // Close overlay drawer after navigation only (not when the user opens it).
  const previousPathname = useRef(location.pathname);
  useEffect(() => {
    if (
      isSidebarOverlay &&
      previousPathname.current !== location.pathname &&
      !shellState.sidebarCollapsed
    ) {
      shellActions.toggleSidebar();
    }
    previousPathname.current = location.pathname;
  }, [
    location.pathname,
    isSidebarOverlay,
    shellState.sidebarCollapsed,
    shellActions,
  ]);

  const handleSidebarClose = (): void => {
    if (!shellState.sidebarCollapsed) {
      shellActions.toggleSidebar();
    }
  };

  // Path Logic Constants
  const isProjectHub = location.pathname === "/" || location.pathname === "";

  const isCaseDetailsPage =
    /\/(?:projects\/[^/]+|[^/]+)\/support\/cases\/[^/]+$/.test(
      location.pathname,
    );

  const isServiceRequestCreatePage = location.pathname.endsWith(
    "/service-requests/create",
  );

  // Resolved: Regex matches details but uses negative lookahead to exclude 'create'
  const isServiceRequestDetailsPage =
    /\/(?:projects\/[^/]+|[^/]+)\/(?:support|operations)\/service-requests\/(?!create$)[^/]+$/.test(
      location.pathname,
    ) && !isServiceRequestCreatePage;

  const isEngagementDetailsPage = location.pathname.includes("/engagements/");

  const isSecurityReportAnalysisDetailsPage =
    /\/(?:projects\/[^/]+|[^/]+)\/security-center\/security-report-analysis\/[^/]+$/.test(
      location.pathname,
    );

  const isVulnerabilityDetailsPage =
    /\/(?:projects\/[^/]+|[^/]+)\/security-center\/[^/]+$/.test(
      location.pathname,
    ) && !location.pathname.includes("security-report-analysis");

  const isPendingUpdatesPage =
    /\/(?:projects\/[^/]+|[^/]+)\/updates\/pending$/.test(location.pathname);

  const isUpdateLevelDetailsPage =
    /\/(?:projects\/[^/]+|[^/]+)\/updates\/pending\/level\/[^/]+$/.test(
      location.pathname,
    );

  // Pages that should have the 'Details' layout (zero padding, etc.)
  const isDetailsStylePage =
    isCaseDetailsPage ||
    isServiceRequestDetailsPage ||
    isEngagementDetailsPage ||
    isSecurityReportAnalysisDetailsPage ||
    isVulnerabilityDetailsPage ||
    isPendingUpdatesPage ||
    isUpdateLevelDetailsPage;
  const hasPortalAccessError = isUnauthorizedError(userDetailsError);

  return (
    <IdleTimeoutProvider>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          height: "100dvh",
          overflow: "hidden",
        }}
      >
        <TopBanner />
        <GlobalNotificationBanner visible={notificationBannerConfig.visible} />
        <HtmlAnnouncementBanner />
        <AppShellLayout
          header={
            <Header
              onToggleSidebar={shellActions.toggleSidebar}
              collapsed={shellState.sidebarCollapsed}
              hideProjectControls={hasPortalAccessError}
            />
          }
          sidebar={
            !isProjectHub && !hasPortalAccessError && !isErrorPageDisplayed ? (
              <SideBar
                collapsed={
                  isSidebarOverlay ? false : shellState.sidebarCollapsed
                }
                expandedMenus={shellState.expandedMenus}
                onSelect={shellActions.setActiveMenuItem}
                onToggleExpand={shellActions.toggleMenu}
              />
            ) : undefined
          }
          sidebarOverlay={isSidebarOverlay}
          sidebarOpen={isSidebarOpen}
          onSidebarClose={handleSidebarClose}
          footer={<Footer />}
        >
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              flex: 1,
              minHeight: 0,
              minWidth: 0,
              width: "100%",
              maxWidth: "100%",
              position: "relative",
              boxSizing: "border-box",
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
                minWidth: 0,
                width: "100%",
                maxWidth: "100%",
                display: "flex",
                flexDirection: "column",
                boxSizing: "border-box",
                ...(isDetailsStylePage ? { minHeight: "60vh" } : {}),
                ...(isAuthLoading
                  ? { p: 0 }
                  : isDetailsStylePage
                    ? { px: 0, pb: 0, pt: 0 }
                    : {
                        px: { xs: 1.5, sm: 2, md: 3 },
                        py: { xs: 2, sm: 2.5, md: 3 },
                      }),
              }}
            >
              {!hasInitialized ? (
                <Box
                  sx={{
                    flex: 1,
                    minHeight: 0,
                    minWidth: 0,
                    width: "100%",
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
              ) : hasPortalAccessError ? (
                <PortalAccessRequiredPage />
              ) : (
                <Box
                  sx={{
                    width: "100%",
                    maxWidth: "100%",
                    minWidth: 0,
                    flex: 1,
                    boxSizing: "border-box",
                  }}
                >
                  {children || (
                    <Outlet
                      context={{
                        sidebarCollapsed: shellState.sidebarCollapsed,
                      }}
                    />
                  )}
                </Box>
              )}
            </Box>
          </Box>
        </AppShellLayout>
      </Box>
    </IdleTimeoutProvider>
  );
}
