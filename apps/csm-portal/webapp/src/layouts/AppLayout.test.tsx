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
import { beforeEach, describe, expect, it, vi } from "vitest";

// AppLayout pulls in a lot of chrome (header, sidebar, banners, the idle
// timeout provider) that isn't relevant to the `showCaseTabs` gating under
// test here, and a couple of those (useAppShell, useAsgardeo) need real
// return shapes to avoid destructuring crashes unrelated to this test.
vi.mock("@wso2/oxygen-ui", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@wso2/oxygen-ui")>()),
  useAppShell: () => ({
    state: { sidebarCollapsed: false, expandedMenus: [] },
    actions: { toggleSidebar: vi.fn(), setActiveMenuItem: vi.fn(), toggleMenu: vi.fn() },
  }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isLoading: false, isSignedIn: true }),
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

// The component under test for the actual bug: renders a marker (real
// `CaseTabsContentHost` would instead mount a restored case tab's page,
// which is what called `useCurrentUser` outside its provider and crashed).
// Standing in for it here isolates the assertion to exactly what
// `AppLayout`'s `showCaseTabs` prop controls: whether this renders at all.
vi.mock("@features/case-tabs/components/CaseTabsWorkspace", () => ({
  CaseTabsContentHost: () => <div data-testid="case-tabs-content-host" />,
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

function renderAppLayout(props: { showCaseTabs?: boolean } = {}) {
  return render(
    <MemoryRouter initialEntries={["/cases/abc123"]}>
      <AppLayout {...props}>
        <div data-testid="routed-page" />
      </AppLayout>
    </MemoryRouter>,
  );
}

describe("AppLayout showCaseTabs gating", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders CaseTabsContentHost by default (CurrentUserProvider is always an ancestor on this path)", () => {
    renderAppLayout();

    expect(screen.getByTestId("case-tabs-content-host")).toBeInTheDocument();
    expect(screen.getByTestId("routed-page")).toBeInTheDocument();
  });

  it("does not render CaseTabsContentHost when showCaseTabs=false, the setting AuthGuard's AuthPendingShell uses because it has no CurrentUserProvider ancestor", () => {
    renderAppLayout({ showCaseTabs: false });

    expect(screen.queryByTestId("case-tabs-content-host")).not.toBeInTheDocument();
    // The routed page (here: AuthPendingShell's own RouteSuspenseFallback
    // stand-in) still renders — only the case-tabs host is suppressed.
    expect(screen.getByTestId("routed-page")).toBeInTheDocument();
  });
});
