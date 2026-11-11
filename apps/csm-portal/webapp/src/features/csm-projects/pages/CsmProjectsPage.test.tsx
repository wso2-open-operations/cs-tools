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

import type { ReactElement } from "react";
import { fireEvent, render as rtlRender, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@testing-library/jest-dom/vitest";
import CsmProjectsPage from "@features/csm-projects/pages/CsmProjectsPage";
import { useSearchProjects } from "@features/csm-projects/api/useSearchProjects";
import type { Project } from "@features/csm-projects/types/csmProjects";

// The backend client reads runtime config at module load, which isn't
// present under vitest — same approach as the sibling accounts/cases page
// tests.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-projects/api/useSearchProjects", () => ({
  useSearchProjects: vi.fn(),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));

vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

const mockedUseSearch = vi.mocked(useSearchProjects);

function render(ui: ReactElement, initialPath = "/customers/projects"): ReturnType<typeof rtlRender> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

function mockResult(overrides: Partial<ReturnType<typeof useSearchProjects>>): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchProjects>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  window.localStorage.clear();
});

// Regression coverage for a real, reported bug: a dashboard tile like
// "Active Onboarding" (resourceType "project", query
// `{onboardingStatus: ["In-Progress"]}`) used to navigate to this page's
// bare, unfiltered path — the click-through dropped the filter entirely
// (see `widgetResourceConfig.ts`'s own `project.buildHref` fix). This page
// is the other half of that fix: it must actually read the
// `?onboardingStatus=` query param the corrected `buildHref` now sends.
describe("CsmProjectsPage — onboardingStatus filter from the URL", () => {
  it("reads a single onboardingStatus value from the URL and includes it in the search request", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />, "/customers/projects?onboardingStatus=In-Progress");

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["In-Progress"] }),
      );
    });
  });

  it("reads multiple comma-separated onboardingStatus values from the URL", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />, "/customers/projects?onboardingStatus=In-Progress,OnHold");

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["In-Progress", "OnHold"] }),
      );
    });
  });

  it("sends no onboardingStatus filter when the URL carries none (the default, unfiltered view)", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />);

    await waitFor(() => expect(mockedUseSearch).toHaveBeenCalled());
    for (const [request] of mockedUseSearch.mock.calls) {
      expect(request).not.toHaveProperty("onboardingStatus");
    }
  });

  it("shows the filter control, reflecting the status read from the URL", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />, "/customers/projects?onboardingStatus=In-Progress");

    expect(await screen.findByText("In Progress")).toBeInTheDocument();
  });

  // The onboardingStatus state used to be seeded only in a useState
  // initializer, which runs once at mount -- so a same-route navigation that
  // doesn't remount the page (e.g. one dashboard tile's click-through
  // followed by another, both landing on /customers/projects) left the
  // filter and the search request stuck on whatever the first mount read,
  // ignoring the URL's new onboardingStatus value. A sibling "navigate"
  // trigger inside the same <Routes> simulates that: the Route element
  // itself never unmounts, only its location/searchParams change.
  it("re-reads onboardingStatus when the URL changes without remounting the page", async () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    function NavTrigger(): ReactElement {
      const navigate = useNavigate();
      return (
        <button onClick={() => navigate("/customers/projects?onboardingStatus=Completed")}>
          go to Completed
        </button>
      );
    }

    rtlRender(
      <MemoryRouter initialEntries={["/customers/projects?onboardingStatus=In-Progress"]}>
        <Routes>
          <Route
            path="/customers/projects"
            element={
              <>
                <NavTrigger />
                <CsmProjectsPage />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["In-Progress"] }),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "go to Completed" }));

    await waitFor(() => {
      expect(mockedUseSearch).toHaveBeenCalledWith(
        expect.objectContaining({ onboardingStatus: ["Completed"] }),
      );
    });
    expect(await screen.findByText("Completed")).toBeInTheDocument();
  });
});

// Regression coverage: this table used to have a fixed 5-column layout with
// no customisation at all, unlike the Cases table (CasesList.tsx) and the
// Accounts table (CsmAccountsPage.tsx), both of which let a caller show/hide/
// reorder columns via ColumnCustomizerButton + useColumnPreferences. Brought
// in line with that convention, plus two new opt-in columns (onboarding
// status/owner) the table previously had nowhere to show at all.
describe("CsmProjectsPage — column customisation", () => {
  it("shows exactly the original 5 columns by default, with onboarding columns hidden", () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />);

    const headerRow = screen.getAllByRole("columnheader").map((h) => h.textContent);
    expect(headerRow).toEqual(["Name", "Project key", "State", "Start", "End"]);
  });

  it("offers a 'Customise projects columns' control", () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });

    render(<CsmProjectsPage />);

    expect(
      screen.getByRole("button", { name: "Customise projects columns" }),
    ).toBeInTheDocument();
  });

  it("shows a project's onboarding status and owner once those columns are toggled on", async () => {
    mockResult({
      data: {
        projects: [
          {
            id: "proj-1",
            name: "Acme Project",
            key: "ACME",
            subscriptionType: "subscription",
            startDate: null,
            endDate: null,
            createdOn: "2024-01-01T00:00:00Z",
            closureState: "open",
            endDateClosureState: null,
            invoiceDueDateClosureState: null,
            complianceViolationClosureState: null,
            complianceViolationDate: null,
            suspensionProcessState: null,
            onboardingStatus: "In-Progress",
            onboardingOwner: { id: "user-2", name: "Jane Doe", email: "jane.doe@example.com" },
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });

    render(<CsmProjectsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    // "Onboarding status" also names the filter's MultiSelectField label, so
    // scope to the picker's own list (aria-labelled with the same button
    // label) to avoid an ambiguous match.
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    fireEvent.click(within(picker).getByText("Onboarding status"));
    fireEvent.click(within(picker).getByText("Onboarding owner"));

    expect(screen.getAllByText("In Progress").length).toBeGreaterThan(0);
    expect(await screen.findByText("Jane Doe")).toBeInTheDocument();
  });

  // Regression (CodeRabbit, PR #2907): the owner cell used to render
  // whenever `onboardingOwner` was set, regardless of `onboardingStatus` —
  // but a "Not-Applicable" status means no active onboarding engagement, the
  // same condition the detail page's own owner cell is gated on.
  it("shows a dash for the owner column when onboardingStatus is Not-Applicable, even with an owner set", async () => {
    mockResult({
      data: {
        projects: [
          {
            id: "proj-1",
            name: "Acme Project",
            key: "ACME",
            subscriptionType: "subscription",
            startDate: null,
            endDate: null,
            createdOn: "2024-01-01T00:00:00Z",
            closureState: "open",
            endDateClosureState: null,
            invoiceDueDateClosureState: null,
            complianceViolationClosureState: null,
            complianceViolationDate: null,
            suspensionProcessState: null,
            onboardingStatus: "Not-Applicable",
            onboardingOwner: { id: "user-2", name: "Jane Doe", email: "jane.doe@example.com" },
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });

    render(<CsmProjectsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    fireEvent.click(within(picker).getByText("Onboarding owner"));

    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
  });

  // Regression (CodeRabbit, PR #2907): the owner cell's UserRefLink had no
  // way to stop its click/keydown from bubbling into the row's own
  // onClick/onKeyDown, so activating the owner's link navigated to the
  // project instead of the owner's own profile page.
  it("clicking the owner link opens the owner's profile, not the project", async () => {
    mockResult({
      data: {
        projects: [
          {
            id: "proj-1",
            name: "Acme Project",
            key: "ACME",
            subscriptionType: "subscription",
            startDate: null,
            endDate: null,
            createdOn: "2024-01-01T00:00:00Z",
            closureState: "open",
            endDateClosureState: null,
            invoiceDueDateClosureState: null,
            complianceViolationClosureState: null,
            complianceViolationDate: null,
            suspensionProcessState: null,
            onboardingStatus: "In-Progress",
            onboardingOwner: { id: "user-2", name: "Jane Doe", email: "jane.doe@example.com" },
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });

    function LocationProbe(): ReactElement {
      const location = useLocation();
      return <div data-testid="location-probe">{location.pathname}</div>;
    }

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    rtlRender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/customers/projects"]}>
          <Routes>
            <Route
              path="/customers/projects"
              element={
                <>
                  <CsmProjectsPage />
                  <LocationProbe />
                </>
              }
            />
            <Route path="/customers/projects/:id" element={<LocationProbe />} />
            <Route path="/people/:id" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    fireEvent.click(within(picker).getByText("Onboarding owner"));
    // Close the popover — MUI's Modal marks the rest of the page aria-hidden
    // while it's open, which hides the owner link from the accessibility
    // tree and makes the getByRole query below fail to find it.
    fireEvent.keyDown(picker, { key: "Escape" });

    fireEvent.click(await screen.findByRole("link", { name: "Jane Doe" }));

    expect(await screen.findByTestId("location-probe")).toHaveTextContent("/people/user-2");
  });
});

const ONBOARDING_DATE_PROJECT: Project = {
  id: "proj-1",
  name: "Acme Project",
  key: "ACME",
  subscriptionType: "subscription",
  startDate: null,
  endDate: null,
  createdOn: "2024-01-01T00:00:00Z",
  closureState: "open",
  endDateClosureState: null,
  invoiceDueDateClosureState: null,
  complianceViolationClosureState: null,
  complianceViolationDate: null,
  suspensionProcessState: null,
  goLivePlanDate: "2026-04-15T00:00:00Z",
  goLiveDate: null,
  onboardingExpiryDate: "2026-12-31T00:00:00Z",
  productionReadinessCompletionDate: "2026-05-20T00:00:00Z",
};

describe("CsmProjectsPage — onboarding date columns", () => {
  it("offers the four onboarding date columns in the customizer, off by default", () => {
    mockResult({ data: { projects: [], total: 0, limit: 20, offset: 0, hasMore: false } });
    render(<CsmProjectsPage />);

    expect(screen.getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "Name",
      "Project key",
      "State",
      "Start",
      "End",
    ]);
    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    for (const label of [
      "Go-live plan date",
      "Go-live date",
      "Onboarding expiry date",
      "Production readiness completion date",
    ]) {
      expect(within(picker).getByText(label)).toBeInTheDocument();
    }
  });

  it("renders the UTC calendar day for each date, and a dash when null", async () => {
    mockResult({
      data: { projects: [ONBOARDING_DATE_PROJECT], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmProjectsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise projects columns" }));
    const picker = screen.getByRole("list", { name: "Customise projects columns" });
    for (const label of [
      "Go-live plan date",
      "Go-live date",
      "Onboarding expiry date",
      "Production readiness completion date",
    ]) {
      fireEvent.click(within(picker).getByText(label));
    }

    // Close the popover: MUI's Modal marks the rest of the page aria-hidden.
    fireEvent.keyDown(picker, { key: "Escape" });

    const row = await screen.findByRole("row", { name: "View project Acme Project" });
    const cells = within(row)
      .getAllByRole("cell")
      .map((c) => c.textContent);
    // Name, key, state, start, end, then the four opted-in dates in order.
    expect(cells.slice(5)).toEqual(["Apr 15, 2026", "—", "Dec 31, 2026", "May 20, 2026"]);
  });
});

describe("CsmProjectsPage — advanced filters", () => {
  function LocationProbe(): ReactElement {
    const location = useLocation();
    return <div data-testid="location-probe">{location.search}</div>;
  }

  function renderWithProbe(initialPath: string): void {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    rtlRender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[initialPath]}>
          <Routes>
            <Route
              path="/customers/projects"
              element={
                <>
                  <CsmProjectsPage />
                  <LocationProbe />
                </>
              }
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
  }

  const EMPTY = { projects: [], total: 0, limit: 20, offset: 0, hasMore: false };
  const lastRequest = (): Record<string, unknown> =>
    mockedUseSearch.mock.calls[mockedUseSearch.mock.calls.length - 1][0] as Record<string, unknown>;

  it("applies state, onboarding status and date ranges from the URL to the search request", async () => {
    mockResult({ data: EMPTY });
    renderWithProbe(
      "/customers/projects?onboardingStatus=In-Progress&closureStates=Open,Suspended&goLiveDateFrom=2026-01-01&goLiveDateTo=2026-01-31&productionReadinessCompletionDateTo=2026-06-30",
    );

    await waitFor(() => {
      expect(lastRequest()).toMatchObject({
        onboardingStatus: ["In-Progress"],
        closureStates: ["Open", "Suspended"],
        goLiveDateFrom: "2026-01-01",
        goLiveDateTo: "2026-01-31",
        productionReadinessCompletionDateTo: "2026-06-30",
      });
    });
    expect(lastRequest()).not.toHaveProperty("goLivePlanDateFrom");
  });

  it("shows the active filters as removable chips and a count on the Filters button", async () => {
    mockResult({ data: EMPTY });
    renderWithProbe(
      "/customers/projects?closureStates=Open&goLiveDateFrom=2026-01-01&goLiveDateTo=2026-01-31",
    );

    expect(await screen.findByText("Project state: Open")).toBeInTheDocument();
    expect(screen.getByText("Go-live date: 2026-01-01 to 2026-01-31")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Filters (2)" })).toBeInTheDocument();
  });

  it("removing a chip drops just that filter from the URL and the request", async () => {
    mockResult({ data: EMPTY });
    renderWithProbe(
      "/customers/projects?closureStates=Open&goLiveDateFrom=2026-01-01&goLiveDateTo=2026-01-31",
    );

    const chip = (await screen.findByText("Go-live date: 2026-01-01 to 2026-01-31")).closest(
      ".MuiChip-root",
    )!;
    fireEvent.click(chip.querySelector(".MuiChip-deleteIcon")!);

    await waitFor(() => {
      expect(screen.getByTestId("location-probe")).toHaveTextContent("?closureStates=Open");
    });
    expect(lastRequest()).toMatchObject({ closureStates: ["Open"] });
    expect(lastRequest()).not.toHaveProperty("goLiveDateFrom");
  });

  it("Clear filters removes every filter but leaves the search box alone", async () => {
    mockResult({ data: EMPTY });
    renderWithProbe(
      "/customers/projects?onboardingStatus=Completed&closureStates=Open&goLiveDateFrom=2026-01-01",
    );

    fireEvent.change(screen.getByLabelText("Search projects by name, key, or subscription type"), {
      target: { value: "acme" },
    });
    fireEvent.click(await screen.findByRole("button", { name: "Clear filters" }));

    await waitFor(() => {
      expect(screen.getByTestId("location-probe")).toHaveTextContent(/^$/);
    });
    expect(screen.queryByRole("button", { name: /^Filters \(/ })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Search projects by name, key, or subscription type")).toHaveValue(
      "acme",
    );
  });

  it("the Advanced builder writes a Project state choice to the URL and the request", async () => {
    mockResult({ data: EMPTY });
    renderWithProbe("/customers/projects");

    fireEvent.click(screen.getByRole("button", { name: "Filters" }));
    fireEvent.click(screen.getByRole("button", { name: "Add filter" }));
    // The first row defaults to Project state; its value is a multi-select.
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /value\(s\)/i }));
    fireEvent.click(screen.getByRole("option", { name: "Suspended" }));

    await waitFor(() => {
      expect(screen.getByTestId("location-probe")).toHaveTextContent("?closureStates=Suspended");
    });
    expect(lastRequest()).toMatchObject({ closureStates: ["Suspended"] });
  });

  it("the Advanced builder offers all the requested fields", async () => {
    mockResult({ data: EMPTY });
    renderWithProbe("/customers/projects");

    fireEvent.click(screen.getByRole("button", { name: "Filters" }));
    fireEvent.click(screen.getByRole("button", { name: "Add filter" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Field" }));
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual([
      "Project state",
      "Onboarding status",
      "Project end date",
      "Onboarding start date",
      "Go-live plan date",
      "Go-live date",
      "Onboarding expiry date",
      "Production readiness completion date",
    ]);
  });
});
