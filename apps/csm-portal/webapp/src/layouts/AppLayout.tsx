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
import { useErrorPageContext } from "@context/error-page/ErrorPageContext";
import { useLocation, Outlet } from "react-router";
import IdleTimeoutProvider from "@providers/IdleTimeoutProvider";
import GlobalNotificationBanner from "@components/notification-banner/GlobalNotificationBanner";
import HtmlAnnouncementBanner from "@components/announcement-banner/HtmlAnnouncementBanner";
import TopBanner from "@components/top-banner/TopBanner";
import Header from "@components/header/Header";
import CsmSideBar from "@components/side-nav-bar/CsmSideBar";

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
      setHasInitialized(true);
    }
  }, [isAuthLoading]);

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
          // overflow: clip (vs hidden) also blocks programmatic/hash-anchor
          // scroll, preventing the layout from shifting horizontally when the
          // URL has a fragment that points to wide content.
          overflow: "clip",
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
                overflow: "clip",
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
                  minWidth: 0,
                  display: "flex",
                  flexDirection: "column",
                  overflowY: "auto",
                  // `auto` (not `clip`) so the user gets a horizontal scrollbar
                  // when page content can't compress further. The companion
                  // `#root main { min-width: 0 }` rule in index.css lets the
                  // <main> shrink to its flex parent, so this scrollbar only
                  // appears when content is genuinely wider than the viewport.
                  overflowX: "auto",
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
                      color="warning"
                      sx={{ width: "80%", maxWidth: 400, height: 4 }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      {loadingMessage}
                    </Typography>
                  </Box>
                ) : (
                  children || <Outlet />
                )}
              </Box>
            </Box>
          </AppShell.Main>
          {/* Footer relocated into CsmSideBar to free vertical space for content. */}
        </AppShell>
      </Box>
    </IdleTimeoutProvider>
  );
}
