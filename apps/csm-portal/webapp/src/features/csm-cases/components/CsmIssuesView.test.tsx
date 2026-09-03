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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import {
  MemoryRouter,
  Route,
  RouterProvider,
  Routes,
  createMemoryRouter,
  useLocation,
} from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));
const useGetCsmCasesMock = vi.fn();
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: (...args: unknown[]) => {
    useGetCsmCasesMock(...args);
    return {
      data: { cases: [], total: 0, hasMore: false },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      dataUpdatedAt: 0,
    };
  },
}));
const casesFilterBarPropsSpy = vi.fn();
vi.mock("@features/csm-cases/components/CasesFilterBar", () => ({
  default: (props: unknown) => {
    casesFilterBarPropsSpy(props);
    return <div>FilterBar</div>;
  },
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  // Forwards `columnCustomizer` (the "Customise columns" trigger, rendered
  // by the real CasesList in a toolbar row above the table) so the column
  // customization tests below can still find/click it, even though this
  // stub renders none of CasesList's own row markup.
  default: ({ columnCustomizer }: { columnCustomizer?: ReactNode }) => (
    <div>
      CasesList
      {columnCustomizer}
    </div>
  ),
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";

beforeEach(() => {
  window.localStorage.clear();
  useGetCsmCasesMock.mockClear();
  casesFilterBarPropsSpy.mockClear();
});

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderAt(initialState: unknown, hideBackButton?: boolean) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: "/cases", state: initialState }]}>
      <Routes>
        <Route
          path="/cases"
          element={<CsmIssuesView title="Cases" hideBackButton={hideBackButton} />}
        />
        <Route path="/dashboard" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("CsmIssuesView back navigation", () => {
  it("renders no Back button when it wasn't reached from a dashboard widget", () => {
    renderAt(null);
    expect(screen.queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
  });

  it("renders a Back button that returns to the dashboard when reached via a dashboard widget's `from` state", () => {
    renderAt({ from: "/dashboard" });

    const backButton = screen.getByRole("button", { name: "Back" });
    fireEvent.click(backButton);

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/dashboard");
  });

  // Regression test: embedding this view as a project's Work items sub-tab
  // still sees the outer page's `from` state (location.state belongs to the
  // route, not this component) and used to render a second, redundant Back
  // button on top of the page-level one. `hideBackButton` must suppress it.
  it("suppresses its own Back button when hideBackButton is set, even with a `from` state present", () => {
    renderAt({ from: "/dashboard" }, true);
    expect(screen.queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
  });
});

describe("CsmIssuesView column customization", () => {
  // Mirrors the real `CsmEngagementsPage`: locked to one case type (so Type
  // is offered but not default-visible — see `isLockedToSingleType`'s doc in
  // `CsmIssuesView`) and severity hidden (Engagements has no severity concept).
  function renderEngagementsLike() {
    return render(
      <MemoryRouter initialEntries={["/engagements"]}>
        <CsmIssuesView
          title="Engagements"
          entityNoun="engagements"
          lockedFilters={{ caseTypes: ["engagement"] }}
          hideSeverityColumn
          enableColumnCustomization
          columnsViewId="engagements"
        />
      </MemoryRouter>,
    );
  }

  it("is off by default (no picker rendered for a plain CsmIssuesView)", () => {
    renderAt(null);
    expect(
      screen.queryByRole("button", { name: /Customise .* columns/ }),
    ).not.toBeInTheDocument();
  });

  it("renders the picker and lists Product/Type/Issue type/Assignee/Reporter/Customer/Created (not Severity) when hideSeverityColumn is set", () => {
    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));

    expect(screen.getByText("Product")).toBeInTheDocument();
    expect(screen.getByText("Type")).toBeInTheDocument();
    expect(screen.getByText("Issue type")).toBeInTheDocument();
    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.getByText("Reporter")).toBeInTheDocument();
    expect(screen.getByText("Customer")).toBeInTheDocument();
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Escalation")).toBeInTheDocument();
    expect(screen.queryByText("Severity")).not.toBeInTheDocument();
  });

  it("defaults every optional column but Product to hidden, since the view is locked to one case type, so a returning user sees no change until they opt in", () => {
    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));

    // Order mirrors `CASE_OPTIONAL_COLUMNS`: Product, Type, Issue type, Assignee,
    // Reporter, Customer, Created, Escalation (Severity is excluded entirely
    // here via hideSeverityColumn).
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(8);
    expect(checkboxes[0]).toBeChecked(); // Product — the one default-visible column
    expect(checkboxes[1]).not.toBeChecked(); // Type
    expect(checkboxes[2]).not.toBeChecked(); // Issue type
    expect(checkboxes[3]).not.toBeChecked(); // Assignee
    expect(checkboxes[4]).not.toBeChecked(); // Reporter
    expect(checkboxes[5]).not.toBeChecked(); // Customer
    expect(checkboxes[6]).not.toBeChecked(); // Created
    expect(checkboxes[7]).not.toBeChecked(); // Escalation
  });

  it("persists a toggled column across a remount for the same user + view", () => {
    const { unmount } = renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));
    fireEvent.click(screen.getByText("Assignee"));
    unmount();

    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[3]).toBeChecked(); // Assignee, toggled on above
  });
});

describe("CsmIssuesView defaultCaseTypes (Support page's case-type default, digiops-cs#2907 follow-up)", () => {
  function renderWithDefault(initialEntry: string) {
    return render(
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/cases"
            element={<CsmIssuesView title="Cases" defaultCaseTypes={["case"]} />}
          />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("seeds caseTypes to the default on a fresh visit with no `types` param at all", () => {
    renderWithDefault("/cases");
    expect(useGetCsmCasesMock).toHaveBeenCalledWith(
      expect.objectContaining({ caseTypes: ["case"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });

  it("does not override an explicit `types` param already in the URL", () => {
    renderWithDefault("/cases?types=service_request");
    expect(useGetCsmCasesMock).toHaveBeenCalledWith(
      expect.objectContaining({ caseTypes: ["service_request"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });

  it("does nothing when hideTypeFilter is set (a locked-type page has no use for a default)", () => {
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <Routes>
          <Route
            path="/cases"
            element={
              <CsmIssuesView
                title="Service Requests"
                lockedFilters={{ caseTypes: ["service_request"] }}
                hideTypeFilter
                defaultCaseTypes={["case"]}
              />
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(useGetCsmCasesMock).toHaveBeenCalledWith(
      expect.objectContaining({ caseTypes: ["service_request"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });

  // Regression: the default used to be re-derived from "does the URL have a
  // `types` param" on every render, not just the first -- so once the user
  // picked an explicit type and then cleared the control back to "every
  // type" (removing `types` from the URL, same as a fresh visit looks),
  // the default silently reasserted itself instead of staying cleared.
  it("does not reassert the default once the user has broadened back to every type, later in the same session", async () => {
    const router = createMemoryRouter(
      [{ path: "/cases", element: <CsmIssuesView title="Cases" defaultCaseTypes={["case"]} /> }],
      { initialEntries: ["/cases"] },
    );
    render(<RouterProvider router={router} />);

    // Fresh visit: default applied.
    await waitFor(() =>
      expect(useGetCsmCasesMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ caseTypes: ["case"] }),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
      ),
    );

    // User picks a different type explicitly.
    await act(async () => { await router.navigate("/cases?types=service_request"); });
    await waitFor(() =>
      expect(useGetCsmCasesMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ caseTypes: ["service_request"] }),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
      ),
    );

    // User clears the type control back to "every type" -- `types` drops out
    // of the URL, identical in shape to the original fresh-visit URL. The
    // default must NOT reassert itself here: the query goes out for every
    // known type (CsmIssuesView's own "empty selection means no type filter"
    // fallback), not back to the single-type default.
    await act(async () => { await router.navigate("/cases"); });
    await waitFor(() =>
      expect(useGetCsmCasesMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ caseTypes: ALL_CASE_TYPES }),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
      ),
    );
  });

  // Regression: the default-consumed ref used to only get set on the branch
  // that actually WROTE the default into the URL -- so a fresh visit that
  // already carried an explicit `types` param (e.g. a direct link to
  // `?types=service_request`) left the ref false, and clearing that type
  // later looked identical to "never touched", wrongly reasserting
  // `defaultCaseTypes` instead of expanding to every type.
  it("does not reassert the default after clearing an explicit type that was already in the URL on the initial visit", async () => {
    const router = createMemoryRouter(
      [{ path: "/cases", element: <CsmIssuesView title="Cases" defaultCaseTypes={["case"]} /> }],
      { initialEntries: ["/cases?types=service_request"] },
    );
    render(<RouterProvider router={router} />);

    // Fresh visit with an explicit type already in the URL: default not applied.
    await waitFor(() =>
      expect(useGetCsmCasesMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ caseTypes: ["service_request"] }),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
      ),
    );

    // User clears the type control -- must expand to every type, not snap
    // back to `defaultCaseTypes`.
    await act(async () => { await router.navigate("/cases"); });
    await waitFor(() =>
      expect(useGetCsmCasesMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ caseTypes: ALL_CASE_TYPES }),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
        expect.anything(),
      ),
    );
  });
});

describe("CsmIssuesView showSeverityFilter override (project Work items tab)", () => {
  it("defaults to hidden when the type filter is unlocked (multi-type view, no lockedFilters.caseTypes)", () => {
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <CsmIssuesView title="Cases" />
      </MemoryRouter>,
    );
    expect(casesFilterBarPropsSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ showSeverityFilter: false }),
    );
  });

  it("shows Severity when the caller passes showSeverityFilter, even though the type filter is unlocked", () => {
    render(
      <MemoryRouter initialEntries={["/projects/p1"]}>
        <CsmIssuesView
          entityNoun="work items"
          lockedFilters={{ projects: ["p1"] }}
          hideProjectFilter
          hideOnboardingStatusFilter
          hideCreTeamFilter
          showSeverityFilter
          typeFilterLabel="Work item type"
        />
      </MemoryRouter>,
    );
    expect(casesFilterBarPropsSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        showSeverityFilter: true,
        hideOnboardingStatusFilter: true,
        hideCreTeamFilter: true,
      }),
    );
  });

  it("does not force-clear a chosen severity out of the query when showSeverityFilter is overridden true", () => {
    render(
      <MemoryRouter initialEntries={["/projects/p1?severities=S1"]}>
        <CsmIssuesView
          entityNoun="work items"
          lockedFilters={{ projects: ["p1"] }}
          showSeverityFilter
        />
      </MemoryRouter>,
    );
    expect(useGetCsmCasesMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ severities: ["S1"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });
});
