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

import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

// Regression test for the "silent re-auth stretches the UI" bug: once the
// app has initialized (a real page has been shown at least once), a LATER
// transient flip of the Asgardeo SDK's `isLoading` flag -- which happens
// when `useAuthApiClient.ts`'s recovery chain calls `signIn()` to force a
// full re-authentication redirect after a dead refresh token, shortly
// before the browser actually navigates away -- must NOT collapse the
// content area's padding back to 0. Reproduced live against a real
// deployment by expiring the stored token and corrupting its refresh
// token, then diffing `getBoundingClientRect()` of the page content across
// the recovery window: the content box's x-inset measurably dropped from
// 88px (64px sidebar + 24px padding) to 64px (padding collapsed to 0) and
// back, i.e. the content visibly snapped to the edges and back -- while
// `useAsgardeo().isLoading` was live-read (not latched) by `AppLayout`'s
// padding sx.
vi.mock("@wso2/oxygen-ui", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@wso2/oxygen-ui")>()),
  useAppShell: () => ({
    state: { sidebarCollapsed: false, expandedMenus: [] },
    actions: { toggleSidebar: vi.fn(), setActiveMenuItem: vi.fn(), toggleMenu: vi.fn() },
  }),
}));

// Mutable across renders so the test can simulate the SDK's `isLoading`
// flipping true -> false (initial sign-in settles) -> true again (a later
// forced-redirect attempt) without remounting the component.
const asgardeoState: { isLoading: boolean; isSignedIn: boolean } = {
  isLoading: true,
  isSignedIn: true,
};
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ ...asgardeoState }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ isVisible: false }),
}));
vi.mock("@context/error-page/ErrorPageContext", () => ({
  useErrorPageContext: () => ({ isErrorPageDisplayed: false }),
}));
vi.mock("@providers/IdleTimeoutProvider", () => ({
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@context/case-tabs/CaseTabsContext", () => ({
  CaseTabsProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@features/case-tabs/components/CaseTabsWorkspace", () => ({
  CaseTabsContentHost: () => null,
  CaseTabStripBar: () => null,
}));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useSyncRecentViewsIdentity: () => undefined,
}));
vi.mock("@components/notification-banner/GlobalNotificationBanner", () => ({
  default: () => null,
}));
vi.mock("@components/announcement-banner/HtmlAnnouncementBanner", () => ({
  default: () => null,
}));
vi.mock("@components/mobile-app-banner/MobileAppBanner", () => ({ default: () => null }));
vi.mock("@components/top-banner/TopBanner", () => ({ default: () => null }));
vi.mock("@components/header/Header", () => ({ default: () => <div data-testid="header" /> }));
vi.mock("@components/side-nav-bar/CsmSideBar", () => ({ default: () => null }));

const { default: AppLayout } = await import("./AppLayout");

describe("AppLayout content padding across a post-init auth loading flip", () => {
  it("keeps the content area's padding once initialized, even when isLoading flips true again later", () => {
    asgardeoState.isLoading = true;
    asgardeoState.isSignedIn = true;

    const { rerender } = render(
      <MemoryRouter initialEntries={["/cases"]}>
        <AppLayout>
          <div data-testid="routed-page" />
        </AppLayout>
      </MemoryRouter>,
    );

    // Cold boot: routed content isn't shown yet (the one-time loading
    // spinner is) -- unaffected by this fix, just establishing the starting
    // state.
    expect(screen.queryByTestId("routed-page")).not.toBeInTheDocument();

    // Initial auth settles -- `hasInitialized` latches true and the routed
    // page appears, padded.
    asgardeoState.isLoading = false;
    rerender(
      <MemoryRouter initialEntries={["/cases"]}>
        <AppLayout>
          <div data-testid="routed-page" />
        </AppLayout>
      </MemoryRouter>,
    );

    const routedPage = screen.getByTestId("routed-page");
    const contentBox = routedPage.parentElement;
    expect(contentBox).not.toBeNull();
    expect(contentBox).toHaveStyle({ padding: "24px" });

    // A later recovery-chain-forced `signIn()` flips `isLoading` true again
    // (see useAuthApiClient.ts's `redirectToSignIn`) well after init. The
    // padding must NOT collapse back to 0 this time -- that collapse (and
    // the snap-back once it resolves) is exactly the reported "stretch".
    asgardeoState.isLoading = true;
    rerender(
      <MemoryRouter initialEntries={["/cases"]}>
        <AppLayout>
          <div data-testid="routed-page" />
        </AppLayout>
      </MemoryRouter>,
    );

    const contentBoxAfter = screen.getByTestId("routed-page").parentElement;
    expect(contentBoxAfter).toHaveStyle({ padding: "24px" });
  });
});
