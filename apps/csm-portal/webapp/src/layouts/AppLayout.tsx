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
import { Box, useAppShell, LinearProgress, Typography } from "@wso2/oxygen-ui";
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
import { CaseTabsProvider } from "@context/case-tabs/CaseTabsContext";
import {
  CaseTabsContentHost,
  CaseTabStripBar,
} from "@features/case-tabs/components/CaseTabsWorkspace";
import { useSyncRecentViewsIdentity } from "@features/csm-recent/hooks/useRecentViews";
import GlobalNotificationBanner from "@components/notification-banner/GlobalNotificationBanner";
import HtmlAnnouncementBanner from "@components/announcement-banner/HtmlAnnouncementBanner";
import MobileAppBanner from "@components/mobile-app-banner/MobileAppBanner";
import TopBanner from "@components/top-banner/TopBanner";
import Header from "@components/header/Header";
import CsmSideBar from "@components/side-nav-bar/CsmSideBar";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import AppShellLayout from "@layouts/AppShellLayout";

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
  /** Forces the header's project controls (search, pin, recent views, sidebar
   * toggle) AND the sidebar itself hidden, even once signed in and
   * initialized — for a full-page state with no real pages to navigate to,
   * search, pin, or revisit (e.g. "not authorized"). Applied synchronously in
   * the same render as `children`, unlike `isErrorPageDisplayed` (context,
   * settles a render later via an effect) — this prop is what actually keeps
   * the sidebar from flashing on screen for one frame before that context
   * update lands. */
  minimalHeader?: boolean;
  /** Gates whether `<CaseTabsContentHost />` (below) renders at all. Defaults
   * to `true` — every normal, signed-in render path has a
   * `CurrentUserProvider` ancestor and case tabs are safe to mount
   * immediately. Must be `false` wherever `AppLayout` is rendered WITHOUT
   * that ancestor (see `AuthGuard.tsx`'s `AuthPendingShell`): a restored
   * open case tab renders `CsmCaseDetailPage`, which calls `useCurrentUser`
   * via `useFindMyOngoingCases`, and that throws outside the provider. Like
   * `minimalHeader`, applied in the same render as `children`, not a render
   * later via an effect. */
  showCaseTabs?: boolean;
}

export default function AppLayout({
  children,
  minimalHeader = false,
  showCaseTabs = true,
}: AppLayoutProps): JSX.Element {
  const location = useLocation();
  const mainContentRef = useRef<HTMLDivElement>(null);
  const { isLoading: isAuthLoading, isSignedIn } = useAsgardeo();
  const { isErrorPageDisplayed } = useErrorPageContext();
  useSyncRecentViewsIdentity();

  // Resets the top-level scroll region on every route change — including a
  // switch between two open case tabs (both are `location.pathname`
  // changes). That's still correct for a GENUINE navigation (a new page
  // should start at the top), and is a no-op for a tab switch specifically:
  // while any case tab is active, this ref's own content never actually
  // overflows it (see `CaseTabsContentHost`'s sizing), because each open
  // tab is its OWN scroll container now — see `CaseTabIsolatedRouter`'s own
  // comment on why that, not a save/restore against THIS ref, is what
  // actually keeps a tab's scroll position across switching away and back.
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
      <CaseTabsProvider>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            height: "100dvh",
            overflow: "hidden",
          }}
        >
          <TopBanner />
          <MobileAppBanner />
          <GlobalNotificationBanner visible={notificationBannerConfig.visible} />
          <HtmlAnnouncementBanner />
          <AppShellLayout
            header={
              <Header
                onToggleSidebar={shellActions.toggleSidebar}
                collapsed={shellState.sidebarCollapsed}
                hideProjectControls={!isSignedIn || !hasInitialized || minimalHeader}
              />
            }
            sidebar={
              hasInitialized && isSignedIn && !isErrorPageDisplayed && !minimalHeader ? (
                <CsmSideBar
                  collapsed={shellState.sidebarCollapsed}
                  expandedMenus={shellState.expandedMenus}
                  onSelect={shellActions.setActiveMenuItem}
                  onToggleExpand={shellActions.toggleMenu}
                />
              ) : undefined
            }
          >
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
                  color="inherit"
                  sx={{
                    color: "primary.main",
                    position: "absolute",
                    top: 0,
                    left: 0,
                    right: 0,
                    zIndex: 1300,
                    height: 3,
                  }}
                />
              )}
              {/* Open in-app case tabs (CaseTabsProvider wraps this whole
                  layout, above): a full-bleed strip above the
                  padded/scrollable content region, like a browser's own tab
                  strip. Renders nothing when no tabs are open. Held off
                  until hasInitialized for the same reason the sidebar is:
                  nothing meaningful to show before auth settles. */}
              {hasInitialized && <CaseTabStripBar />}
              <Box
                ref={mainContentRef}
                sx={{
                  flex: 1,
                  minHeight: 0,
                  minWidth: 0,
                  display: "flex",
                  flexDirection: "column",
                  overflow: "auto",
                  // Zero padding only for the initial cold-boot loading spinner
                  // below (centers it flush, no boxed inset). Gated on the
                  // `hasInitialized` LATCH, not the live `isAuthLoading` flag
                  // directly: once the app has initialized once, `isAuthLoading`
                  // can still flip true again later — e.g. the recovery chain in
                  // `useAuthApiClient.ts` calls `signIn()` for a forced
                  // re-authentication redirect after a dead refresh token, and
                  // the SDK sets its loading flag before that redirect actually
                  // navigates away. Reading the live flag here made the padding
                  // (and therefore the content width/position) collapse to 0 and
                  // snap back for that whole window on every such recovery,
                  // visible as the content area suddenly stretching edge-to-edge
                  // and back — reproduced and measured via getBoundingClientRect
                  // during a forced-expiry repro; see the task notes for the
                  // exact before/after rects. `hasInitialized` already exists as
                  // the one-way "have we ever finished initial auth" latch (see
                  // its own effect above) and is the correct gate for a
                  // one-time-only layout decision like this.
                  ...(hasInitialized ? { p: 3 } : { p: 0 }),
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
                      color="inherit"
                      sx={{ color: "primary.main", width: "80%", maxWidth: 400, height: 4 }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      {loadingMessage}
                    </Typography>
                  </Box>
                ) : (
                  <Suspense fallback={<RouteSuspenseFallback />}>
                    {/* Every open case tab's page, kept alive and hidden via
                        CSS unless it's both the active tab and the current
                        route is a case-detail route — see
                        CaseTabsContentHost's own doc comment. Sits alongside
                        (not instead of) children/<Outlet/>: a case route's
                        own element now renders nothing itself once its tab
                        is open (see CaseDetailRouteSync), so there's no
                        double-render. */}
                    {showCaseTabs && <CaseTabsContentHost />}
                    {children || <Outlet />}
                  </Suspense>
                )}
              </Box>
            </Box>
          </AppShellLayout>
        </Box>
      </CaseTabsProvider>
    </IdleTimeoutProvider>
  );
}
