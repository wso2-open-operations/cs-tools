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

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// Same reasoning as CsmIssuesView.workStateUrl.test.tsx: the bug under test
// only reproduces through the real round trip of a filter-bar interaction ->
// CsmIssuesView's setFilters -> the URL -> re-reading the URL back into
// `filters`, so this keeps the real CasesFilterBar (and CasesList) mounted
// rather than mocking it away.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ teams: [] }), get: vi.fn() }),
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
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: () => ({
    data: { cases: [], total: 0, hasMore: false },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
  }),
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

/** Exposes the current URL search string so a test can assert on it
 * directly -- `window.location` doesn't reflect MemoryRouter's history. */
function LocationSearchProbe() {
  const location = useLocation();
  return <div data-testid="search-probe">{location.search}</div>;
}

function renderAt(initialUrl: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialUrl]}>
        <Routes>
          <Route
            path="/cases"
            element={
              <>
                <CsmIssuesView title="Cases" />
                <LocationSearchProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// Regression test for: removing the last "Advanced filters" row (a field
// with no typed `CasesFilters` slot, e.g. "Created by is me") — or the last
// "OR groups" branch — silently did nothing to the URL, live-verified via
// the debug Chrome. Root cause: same class of bug the workStates/
// onboardingStatuses regressions above already document — CsmIssuesView's
// FILTER_PARAM_KEYS (the URL keys it deletes before rewriting from the next
// filter state) was missing "af" and "anyOf", so an empty
// `next.advancedFilters`/`next.anyOfBranches` never actually cleared the
// stale `af`/`anyOf` URL param, and the next render read the stale row/
// branch straight back in (`writeCasesFiltersToUrl` only ever *sets* these
// two once there's at least one row/branch again — it never explicitly
// clears them, so an explicit `delete` first is required, same as every
// other filter field).
describe("CsmIssuesView + real CasesFilterBar — Advanced filters row clears fully from the URL", () => {
  it("removing the only advancedFilters row clears the `af` param, not just the in-memory value", () => {
    renderAt('/cases?af=%5B%5B%22createdBy%22%2C%22eq%22%5D%5D');

    // An advanced-only field with no typed `CasesFilters` slot forces
    // Advanced mode on mount (see `isSimpleRepresentable`), where the row
    // itself (not a chip -- Advanced mode renders zero chips, see
    // `buildActiveFilterChips`'s doc comment in `CasesFilterBar.tsx`) is
    // the only visible/removable UI for it.
    expect(screen.getByRole("button", { name: "Remove filter row" })).toBeInTheDocument();
    expect(screen.getByTestId("search-probe").textContent).toContain("af=");

    fireEvent.click(screen.getByRole("button", { name: "Remove filter row" }));

    expect(screen.queryByRole("button", { name: "Remove filter row" })).not.toBeInTheDocument();
    expect(screen.getByTestId("search-probe").textContent).not.toContain("af=");
  });
});

describe("CsmIssuesView + real CasesFilterBar — OR groups clears fully from the URL", () => {
  it("removing the only OR-group branch clears the `anyOf` param, not just the in-memory value", () => {
    renderAt('/cases?anyOf=%5B%5B%5B%22type%22%2C%5B%22case%22%5D%5D%5D%5D');

    // The branch itself (rendered by `AnyOfGroupsBuilder`, not a chip --
    // Advanced mode renders zero chips, see `buildActiveFilterChips`'s doc
    // comment in `CasesFilterBar.tsx`) is the only visible/removable UI for
    // it.
    expect(screen.getByText("Group 1")).toBeInTheDocument();
    expect(screen.getByTestId("search-probe").textContent).toContain("anyOf=");

    fireEvent.click(screen.getByRole("button", { name: "Remove OR group 1" }));

    expect(screen.queryByText("Group 1")).not.toBeInTheDocument();
    expect(screen.getByTestId("search-probe").textContent).not.toContain("anyOf=");
  });
});
