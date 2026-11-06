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

import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";

// Records the props CsmSideBar hands the real Sidebar on each render, without
// reassigning an outer-scope variable during render (a mock function call is
// not a render side effect the way a bare assignment is).
const sidebarPropsSpy = vi.fn();

interface CapturedSidebarProps {
  activeItem?: string;
  expandedMenus?: Record<string, boolean>;
  onSelect?: (id: string) => void;
  onToggleExpand?: (id: string) => void;
}

function lastSidebarProps(): CapturedSidebarProps {
  return (sidebarPropsSpy.mock.calls.at(-1)?.[0] ?? {}) as CapturedSidebarProps;
}

function lastActiveItem(): string | undefined {
  return lastSidebarProps().activeItem;
}

const navigateMock = vi.fn();
// The signed-in user's roles; cs_engineer unless a describe block switches it.
const currentRoles = vi.hoisted(() => ({ value: ["cs_engineer"] as string[] }));
// The id keys the user's Feature Preview values in localStorage
// (featurePreview.ts); nothing is stored under it unless a test does so.
const CURRENT_USER_ID = "user-1";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: CURRENT_USER_ID, roles: currentRoles.value },
    isLoading: false,
    isError: false,
    error: null,
  }),
}));
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

// The real Sidebar's internal DOM/selection/expand-collapse machinery isn't
// this test's concern -- only what CsmSideBar computed and handed it
// (activeItem, expandedMenus, onSelect) and what it rendered as nested
// children. Every compound sub-component it renders (`Sidebar.Nav`,
// `.Item`, ...) is stubbed to a passthrough so CsmSideBar's own JSX still
// resolves and nested `Sidebar.Item`s render their labels for assertions.
vi.mock("@wso2/oxygen-ui", async () => {
  const actual = await vi.importActual<typeof import("@wso2/oxygen-ui")>("@wso2/oxygen-ui");
  const Passthrough = ({ children }: { children?: ReactNode }) => <>{children}</>;
  // A real (if bare) DOM element per item/label, not a fragment passthrough —
  // so a nested item's own text is its own element's content for
  // `getByText`, distinct from its parent's aggregate text.
  const MockItem = ({ id, children }: { id?: string; children?: ReactNode }) => (
    <div data-item-id={id}>{children}</div>
  );
  const MockItemLabel = ({ children }: { children?: ReactNode }) => <span>{children}</span>;
  function MockSidebar({
    activeItem,
    expandedMenus,
    onSelect,
    onToggleExpand,
    children,
  }: CapturedSidebarProps & { children?: ReactNode }) {
    sidebarPropsSpy({ activeItem, expandedMenus, onSelect, onToggleExpand });
    return <div data-testid="sidebar">{children}</div>;
  }
  MockSidebar.Nav = Passthrough;
  MockSidebar.Category = Passthrough;
  MockSidebar.CategoryLabel = Passthrough;
  MockSidebar.Item = MockItem;
  MockSidebar.ItemIcon = Passthrough;
  MockSidebar.ItemLabel = MockItemLabel;
  MockSidebar.ItemBadge = Passthrough;
  MockSidebar.Footer = Passthrough;
  return { ...actual, Sidebar: MockSidebar };
});

import CsmSideBar from "@components/side-nav-bar/CsmSideBar";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import { previewStorageKey } from "@config/featurePreview";

const LAST_SECTION_KEY = "csm.sidebar.lastSection";

function renderAt(path: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <CsmSideBar collapsed={false} />
    </MemoryRouter>,
  );
}

function renderAtWithExpanded(
  path: string,
  expandedMenus: Record<string, boolean>,
): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <CsmSideBar collapsed={false} expandedMenus={expandedMenus} />
    </MemoryRouter>,
  );
}

describe("CsmSideBar — active section on routes with no owning nav section", () => {
  beforeEach(() => {
    sidebarPropsSpy.mockClear();
    navigateMock.mockClear();
    sessionStorage.clear();
  });

  it("defaults to dashboard on a first-ever visit with no remembered section", () => {
    renderAt("/people/user-1");
    expect(lastActiveItem()).toBe("dashboard");
  });

  it("highlights and remembers the owning section for a route that has one", () => {
    renderAt("/admin/users");
    expect(lastActiveItem()).toBe("admin");
    expect(sessionStorage.getItem(LAST_SECTION_KEY)).toBe("admin");
  });

  // Regression test: a full page reload on /people/:id (e.g. a user profile,
  // linked from all over the app and not owned by any nav section) used to
  // always fall back to the hardcoded "dashboard" ref default, even if the
  // user had just come from Settings > Users. It must instead read back
  // whichever section was active before the reload.
  it("resolves to the last section remembered before a reload, not dashboard", () => {
    sessionStorage.setItem(LAST_SECTION_KEY, "admin");
    renderAt("/people/user-1");
    expect(lastActiveItem()).toBe("admin");
  });

  // Regression test: visiting a submenu child route (e.g.
  // `/operations/incidents`) used to persist the *child's* dotted id
  // (`operations.incidents`) as the remembered section, not the owning
  // section (`operations`). A later visit to a section-less route then read
  // that stale child id back as the fallback, lighting up the old child and
  // auto-expanding Operations even though nothing about the new route has
  // anything to do with it.
  it("remembers the owning section, not the child's own dotted id, after visiting a submenu child route", () => {
    renderAt("/operations/incidents");
    expect(sessionStorage.getItem(LAST_SECTION_KEY)).toBe("operations");

    sidebarPropsSpy.mockClear();
    renderAt("/people/user-1");
    expect(lastActiveItem()).toBe("operations");
  });

  // Regression test: sessionStorage survives a reload, so a tab open across the
  // deploy that fixed the above can still hold a dotted child id written by the
  // older build. Reading it back must yield the owning section, not the stale
  // child id, on the very first render.
  it("normalises a dotted child id left in storage by an older build", () => {
    sessionStorage.setItem(LAST_SECTION_KEY, "operations.incidents");
    renderAt("/people/user-1");
    expect(lastActiveItem()).toBe("operations");
  });
});

describe("CsmSideBar — Operations/Security Center submenu", () => {
  beforeEach(() => {
    sidebarPropsSpy.mockClear();
    navigateMock.mockClear();
    sessionStorage.clear();
  });

  it("renders Operations' children as nested items in the rail", () => {
    renderAt("/dashboard");
    expect(screen.getByText("Service requests")).toBeInTheDocument();
    expect(screen.getByText("Change requests")).toBeInTheDocument();
    expect(screen.getByText("Incidents")).toBeInTheDocument();
    expect(screen.getByText("Problem management")).toBeInTheDocument();
  });

  it("renders Security Center's children as nested items in the rail", () => {
    renderAt("/dashboard");
    expect(screen.getByText("Security reports")).toBeInTheDocument();
    expect(screen.getByText("Vulnerabilities")).toBeInTheDocument();
  });

  // Regression test: a submenu child used to render its label only, with no
  // icon at all -- unlike every top-level section and every admin.
  // user-management tile, all of which carry one.
  it("renders an icon for each Operations/Security Center submenu child, not just its label", () => {
    const { container } = renderAt("/dashboard");
    const childIds = [
      "operations.service-requests",
      "operations.change-requests",
      "operations.incidents",
      "operations.problems",
      "security-center.reports",
      "security-center.vulnerabilities",
    ];
    for (const id of childIds) {
      const item = container.querySelector(`[data-item-id="${id}"]`);
      expect(item).not.toBeNull();
      expect(item?.querySelector("svg")).not.toBeNull();
    }
  });

  it("does not extend the submenu treatment to Customers/Settings — their children stay out of the rail", () => {
    renderAt("/dashboard");
    // Customers/Settings still switch tabs via their own in-page/route strip
    // (`useRouteTabs`), untouched by this change — their children must not
    // gain rail entries as a side effect of the Operations/Security work.
    expect(screen.queryByText("Accounts")).not.toBeInTheDocument();
    expect(screen.queryByText("User management")).not.toBeInTheDocument();
  });

  it("highlights the active child tab, not just the owning section, on a fresh load", () => {
    renderAt("/operations/incidents");
    expect(lastActiveItem()).toBe("operations.incidents");
  });

  it("auto-expands Operations on a fresh load when one of its children is active", () => {
    renderAt("/operations/incidents");
    expect(lastSidebarProps().expandedMenus?.operations).toBe(true);
  });

  it("auto-expands Security Center on a fresh load when one of its children is active", () => {
    renderAt("/security-center/vulnerabilities");
    expect(lastSidebarProps().expandedMenus?.["security-center"]).toBe(true);
  });

  it("does not force-expand Operations when a different section is active", () => {
    renderAt("/dashboard");
    expect(lastSidebarProps().expandedMenus?.operations).toBeFalsy();
  });

  // Regression: collapsing a submenu section while still on one of its own
  // child pages used to be a no-op. `toggleMenu` correctly flipped
  // `expandedMenus.operations` to `false`, but the auto-expand memo below
  // re-derived it back to `true` on every render since `activeItem` still
  // started with "operations." (the route hadn't changed) — the chevron
  // click had no visible effect at all.
  it("respects an explicit collapse even while one of the section's own children is still active", () => {
    renderAtWithExpanded("/operations/incidents", { operations: false });
    expect(lastSidebarProps().expandedMenus?.operations).toBe(false);
  });

  it("respects an explicit expand the same way, without needing the auto-expand fallback", () => {
    renderAtWithExpanded("/operations/incidents", { operations: true });
    expect(lastSidebarProps().expandedMenus?.operations).toBe(true);
  });

  it("navigates to the real path-segment route for a selected submenu child, not the legacy ?tab= href", () => {
    renderAt("/dashboard");
    lastSidebarProps().onSelect?.("operations.incidents");
    expect(navigateMock).toHaveBeenCalledWith("/operations/incidents");
  });

  it("navigates a Security Center child selection to its own path-segment route", () => {
    renderAt("/dashboard");
    lastSidebarProps().onSelect?.("security-center.vulnerabilities");
    expect(navigateMock).toHaveBeenCalledWith("/security-center/vulnerabilities");
  });

  it("does not navigate for a flat top-level section id selected directly (its own Link already handles it)", () => {
    renderAt("/dashboard");
    lastSidebarProps().onSelect?.("engagements");
    expect(navigateMock).not.toHaveBeenCalled();
  });
});

// A viewer who is not also a CS engineer/admin gets Customer Health inside a
// Customers dropdown instead of as its own top-level entry.
describe("CsmSideBar — Customer Health under Customers for a viewer", () => {
  beforeEach(() => {
    currentRoles.value = ["viewer"];
    sidebarPropsSpy.mockClear();
    navigateMock.mockClear();
    sessionStorage.clear();
  });

  afterEach(() => {
    currentRoles.value = ["cs_engineer"];
  });

  it("renders Customers as a dropdown with Accounts, Projects and Customer Health", () => {
    const { container } = renderAt("/dashboard");
    const customers = container.querySelector('[data-item-id="customers"]');
    expect(customers).not.toBeNull();
    const nested = Array.from(customers!.querySelectorAll("[data-item-id]")).map((el) =>
      el.getAttribute("data-item-id"),
    );
    expect(nested).toEqual(["customers.accounts", "customers.projects", "customers.customer-health"]);
    expect(customers!.querySelector('[data-item-id="customers.customer-health"] svg')).not.toBeNull();
  });

  it("drops the top-level Customer Health entry", () => {
    const { container } = renderAt("/dashboard");
    expect(container.querySelector('[data-item-id="customer-health"]')).toBeNull();
    expect(screen.getAllByText("Customer Health")).toHaveLength(1);
  });

  it("highlights Customer Health in the dropdown, and opens it, on the dashboard and on an account's detail", () => {
    renderAt("/customer-health");
    expect(lastActiveItem()).toBe("customers.customer-health");
    expect(lastSidebarProps().expandedMenus?.customers).toBe(true);

    sidebarPropsSpy.mockClear();
    renderAt("/customer-health/account/acct-1");
    expect(lastActiveItem()).toBe("customers.customer-health");
    expect(lastSidebarProps().expandedMenus?.customers).toBe(true);
  });

  it("highlights Accounts or Projects in the dropdown on their own pages", () => {
    renderAt("/customers/accounts");
    expect(lastActiveItem()).toBe("customers.accounts");
    sidebarPropsSpy.mockClear();
    renderAt("/customers/projects/proj-1");
    expect(lastActiveItem()).toBe("customers.projects");
  });

  it("remembers Customers, not the dotted child id, as the last section", () => {
    renderAt("/customer-health");
    expect(sessionStorage.getItem(LAST_SECTION_KEY)).toBe("customers");
  });

  it("navigates to each dropdown entry's page when it is selected", () => {
    renderAt("/dashboard");
    lastSidebarProps().onSelect?.("customers.customer-health");
    expect(navigateMock).toHaveBeenLastCalledWith("/customer-health");
    lastSidebarProps().onSelect?.("customers.accounts");
    expect(navigateMock).toHaveBeenLastCalledWith("/customers/accounts");
  });

  it("keeps a CS engineer who also holds viewer on the top-level entry, with Customers as a plain link", () => {
    currentRoles.value = ["cs_engineer", "viewer"];
    const { container } = renderAt("/customer-health");
    expect(container.querySelector('[data-item-id="customer-health"]')).not.toBeNull();
    expect(container.querySelector('[data-item-id="customers.customer-health"]')).toBeNull();
    expect(screen.queryByText("Accounts")).not.toBeInTheDocument();
    expect(lastActiveItem()).toBe("customer-health");
  });
});

// The new dropdown-child highlight is only for a section that opens as a
// dropdown for this user; a CS engineer keeps highlighting the section itself.
describe("CsmSideBar — route-tab sections for a CS engineer", () => {
  beforeEach(() => {
    currentRoles.value = ["cs_engineer"];
    sidebarPropsSpy.mockClear();
    sessionStorage.clear();
  });

  it("highlights Customers, not its tab, on Accounts and Projects pages", () => {
    renderAt("/customers/accounts");
    expect(lastActiveItem()).toBe("customers");
    sidebarPropsSpy.mockClear();
    renderAt("/customers/projects/proj-1");
    expect(lastActiveItem()).toBe("customers");
  });

  it("highlights Settings, not its tab, on a Settings page", () => {
    renderAt("/admin/user-management");
    expect(lastActiveItem()).toBe("admin");
    sidebarPropsSpy.mockClear();
    renderAt("/admin/user-management/users");
    expect(lastActiveItem()).toBe("admin");
  });
});

describe("CsmSideBar — collapsing a dropdown that opened by itself", () => {
  const toggleSpy = vi.fn();

  function renderWithToggle(path: string, expandedMenus?: Record<string, boolean>): void {
    render(
      <MemoryRouter initialEntries={[path]}>
        <CsmSideBar collapsed={false} expandedMenus={expandedMenus} onToggleExpand={toggleSpy} />
      </MemoryRouter>,
    );
  }

  beforeEach(() => {
    currentRoles.value = ["viewer"];
    sidebarPropsSpy.mockClear();
    toggleSpy.mockClear();
    sessionStorage.clear();
  });

  afterEach(() => {
    currentRoles.value = ["cs_engineer"];
  });

  // toggleMenu flips the stored value; for an auto-opened section that is
  // undefined -> true (no visible change), so it is flipped twice to close.
  it("closes an auto-opened section on the first click", () => {
    renderWithToggle("/customer-health", {});
    lastSidebarProps().onToggleExpand?.("customers");
    expect(toggleSpy).toHaveBeenCalledTimes(2);
    expect(toggleSpy).toHaveBeenNthCalledWith(1, "customers");
    expect(toggleSpy).toHaveBeenNthCalledWith(2, "customers");
  });

  it("toggles once when the section's state was set explicitly", () => {
    renderWithToggle("/customer-health", { customers: true });
    lastSidebarProps().onToggleExpand?.("customers");
    expect(toggleSpy).toHaveBeenCalledTimes(1);
  });

  it("toggles once when the section is not auto-opened", () => {
    renderWithToggle("/dashboard", {});
    lastSidebarProps().onToggleExpand?.("customers");
    expect(toggleSpy).toHaveBeenCalledTimes(1);
  });
});

// The shipped config.js.example marks Customers WIP: a viewer must then keep
// the working top-level Customer Health entry.
describe("CsmSideBar — viewer when a deployment marks Customers WIP", () => {
  function setOverrides(value: unknown): void {
    window.config = {
      ...window.config,
      CSM_PORTAL_FEATURE_OVERRIDES: value,
    } as Window["config"];
    resetFeatureStatesForTests();
  }

  beforeEach(() => {
    currentRoles.value = ["viewer"];
    sidebarPropsSpy.mockClear();
    sessionStorage.clear();
    setOverrides({ customers: "wip" });
  });

  afterEach(() => {
    currentRoles.value = ["cs_engineer"];
    setOverrides(undefined);
  });

  it("shows the top-level Customer Health entry and no shortcut", () => {
    const { container } = renderAt("/dashboard");
    expect(container.querySelector('[data-item-id="customer-health"]')).not.toBeNull();
    expect(container.querySelector('[data-item-id="customers.customer-health"]')).toBeNull();
  });

  it("highlights the top-level entry on the Customer Health page", () => {
    renderAt("/customer-health");
    expect(lastActiveItem()).toBe("customer-health");
  });
});

// An admin's Feature Preview values (featurePreview.ts: browser-local, keyed
// by user id) reach the sidebar's dropdown and highlight logic, but never
// make the viewer-only shortcut appear, and do nothing for anyone else.
describe("CsmSideBar — Feature Preview and the Customers shortcut", () => {
  function setOverrides(value: unknown): void {
    window.config = {
      ...window.config,
      CSM_PORTAL_FEATURE_OVERRIDES: value,
    } as Window["config"];
    resetFeatureStatesForTests();
  }

  function setPreview(overrides: Record<string, boolean>): void {
    localStorage.setItem(previewStorageKey(CURRENT_USER_ID), JSON.stringify(overrides));
  }

  beforeEach(() => {
    sidebarPropsSpy.mockClear();
    sessionStorage.clear();
    localStorage.clear();
  });

  afterEach(() => {
    currentRoles.value = ["cs_engineer"];
    setOverrides(undefined);
    localStorage.clear();
  });

  it("shows an admin previewing a hidden Customer Health the top-level entry, never the shortcut", () => {
    currentRoles.value = ["admin"];
    setOverrides({ "customer-health": "hidden" });
    setPreview({ "customer-health": true, "customers.customer-health": true });
    const { container } = renderAt("/customer-health");
    expect(container.querySelector('[data-item-id="customer-health"]')).not.toBeNull();
    expect(container.querySelector('[data-item-id="customers.customer-health"]')).toBeNull();
    // Customers stays a plain link for an admin: no dropdown entries.
    expect(screen.queryByText("Accounts")).not.toBeInTheDocument();
    expect(lastActiveItem()).toBe("customer-health");
  });

  it("ignores preview values stored for a viewer", () => {
    currentRoles.value = ["viewer"];
    setOverrides({ customers: "wip" });
    setPreview({ customers: true, "customers.accounts": true, "customers.customer-health": true });
    const { container } = renderAt("/customer-health");
    // Customers is still WIP for the viewer, so they keep the top-level entry.
    expect(container.querySelector('[data-item-id="customer-health"]')).not.toBeNull();
    expect(container.querySelector('[data-item-id="customers.customer-health"]')).toBeNull();
    expect(lastActiveItem()).toBe("customer-health");
  });

  it("does not surface a hidden Customers for an admin through a preview value for the shortcut", () => {
    currentRoles.value = ["admin"];
    setOverrides({ customers: "hidden" });
    setPreview({ "customers.customer-health": true });
    const first = renderAt("/dashboard");
    expect(first.container.querySelector('[data-item-id="customers"]')).toBeNull();
    first.unmount();

    // Previewing one of its real tabs does surface it.
    setPreview({ "customers.accounts": true });
    const second = renderAt("/dashboard");
    expect(second.container.querySelector('[data-item-id="customers"]')).not.toBeNull();
  });

  it("marks a top-level item shown only via preview with a Preview badge", () => {
    currentRoles.value = ["admin"];
    setOverrides({ "customer-health": "hidden" });
    setPreview({ "customer-health": true, "customers.customer-health": true });
    const { container } = renderAt("/customer-health");
    const item = container.querySelector('[data-item-id="customer-health"]');
    expect(item).not.toBeNull();
    expect(item).toHaveTextContent("Preview");
  });

  it("marks BOTH a submenu parent (promoted purely via a previewed descendant) and the previewed child with a Preview badge", () => {
    currentRoles.value = ["admin"];
    setOverrides({ operations: "hidden" });
    setPreview({ "operations.incidents": true });
    const { container } = renderAt("/dashboard");

    const parent = container.querySelector('[data-item-id="operations"]');
    expect(parent).not.toBeNull();
    // The parent carries no override/preview value of its own -- only its
    // child does -- so this is specifically the descendant-promotion path.
    expect(parent).toHaveTextContent("Preview");

    const child = container.querySelector('[data-item-id="operations.incidents"]');
    expect(child).not.toBeNull();
    expect(child).toHaveTextContent("Preview");

    // A sibling child that was never previewed stays hidden entirely (it
    // inherits "hidden" from its still-restricted parent, same as any other
    // un-previewed descendant of a hidden section) -- not rendered with no
    // badge, simply not rendered at all.
    expect(container.querySelector('[data-item-id="operations.problems"]')).toBeNull();
  });

  it("never shows a Preview badge for a non-admin, even with a stored preview value", () => {
    currentRoles.value = ["cs_engineer"];
    setOverrides({ "customer-health": "hidden" });
    setPreview({ "customer-health": true });
    const { container } = renderAt("/dashboard");
    expect(container.querySelector('[data-item-id="customer-health"]')).toBeNull();
  });
});
