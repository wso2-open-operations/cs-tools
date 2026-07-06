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
import {
  type JSX,
  type ReactNode,
  Suspense,
  useRef,
  useEffect,
  useState,
} from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorPageContext } from "@context/error-page/ErrorPageContext";
import { useLocation, Outlet } from "react-router";
import IdleTimeoutProvider from "@providers/IdleTimeoutProvider";
import GlobalNotificationBanner from "@components/notification-banner/GlobalNotificationBanner";
import HtmlAnnouncementBanner from "@components/announcement-banner/HtmlAnnouncementBanner";
import TopBanner from "@components/top-banner/TopBanner";
import Header from "@components/header/Header";
import CsmSideBar from "@components/side-nav-bar/CsmSideBar";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";

const SIDEBAR_COLLAPSED_KEY = "csm.sidebar.collapsed";

function getSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

function setSidebarCollapsed(collapsed: boolean): void {
  try {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? "1" : "0");
  } catch {
    /* ignore */
  }
}

interface AppLayoutProps {
  children?: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps): JSX.Element {
  const location = useLocation();
  const mainContentRef = useRef<HTMLDivElement>(null);
  const { isLoading: isAuthLoading, isSignedIn } = useAsgardeo();
  const { isErrorPageDisplayed } = useErrorPageContext();

  useEffect(() => {
    if (mainContentRef.current) {
      mainContentRef.current.scrollTop = 0;
    }
  }, [location.pathname]);

  const { state: shellState, actions: shellActions } = useAppShell({
    initialCollapsed: getSidebarCollapsed(),
  });

  const { isVisible } = useLoader();
  const isLoginCallback =
    new URLSearchParams(location.search).has("code") &&
    new URLSearchParams(location.search).has("state");

  const [hasInitialized, setHasInitialized] = useState(false);
  const [loadingMessage, setLoadingMessage] = useState<string>(
    isLoginCallback ? "Authenticating…" : "Loading…",
  );

  useEffect(() => {
    if (!isAuthLoading) {
      // One-way init latch: flips to true once auth settles and stays there.
      // Effect-driven by design (must re-render on the transition).
      // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional init latch
      setHasInitialized(true);
    }
  }, [isAuthLoading]);

  useEffect(() => {
    if (!isAuthLoading || !isLoginCallback) return;

    // Re-asserts the message when the callback effect (re)runs; the staged
    // timers below legitimately setState from async callbacks.
    // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs message to auth/callback state
    setLoadingMessage("Authenticating…");
    const t1 = setTimeout(() => setLoadingMessage("Fetching user info…"), 1500);
    const t2 = setTimeout(() => setLoadingMessage("Please wait…"), 3000);
    return () => {
      clearTimeout(t1);
      clearTimeout(t2);
    };
  }, [isAuthLoading, isLoginCallback]);

  useEffect(() => {
    setSidebarCollapsed(shellState.sidebarCollapsed);
  }, [shellState.sidebarCollapsed]);

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
        <AppShell sx={{ flex: 1, minHeight: 0, height: "auto" }}>
          <AppShell.Navbar>
            <Header
              onToggleSidebar={shellActions.toggleSidebar}
              collapsed={shellState.sidebarCollapsed}
              hideProjectControls={!isSignedIn || !hasInitialized}
            />
          </AppShell.Navbar>

          {hasInitialized && isSignedIn && !isErrorPageDisplayed && (
            <AppShell.Sidebar>
              <CsmSideBar
                collapsed={shellState.sidebarCollapsed}
                expandedMenus={shellState.expandedMenus}
                onSelect={shellActions.setActiveMenuItem}
                onToggleExpand={shellActions.toggleMenu}
              />
            </AppShell.Sidebar>
          )}

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
                  color="primary"
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
                  display: "flex",
                  flexDirection: "column",
                  overflowY: "auto",
                  overflowX: "hidden",
                  ...(isAuthLoading ? { p: 0 } : { p: 3 }),
                }}
              >
                {!hasInitialized ? (
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
                      color="primary"
                      sx={{ width: "80%", maxWidth: 400, height: 4 }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      {loadingMessage}
                    </Typography>
                  </Box>
                ) : (
                  <Suspense fallback={<RouteSuspenseFallback />}>
                    {children || <Outlet />}
                  </Suspense>
                )}
              </Box>
            </Box>
          </AppShell.Main>
        </AppShell>
      </Box>
    </IdleTimeoutProvider>
  );
}
