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

import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// Unlike CsmIssuesView.test.tsx, this file keeps the REAL CasesFilterBar (and
// CasesList) mounted -- the bug under test only reproduces through the real
// round trip of a filter bar interaction -> CsmIssuesView's setFilters ->
// the URL -> re-reading the URL back into `filters`.
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
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));
const searchSpy = vi.hoisted(() => vi.fn());
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: (...args: unknown[]) => {
    searchSpy(...args);
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

function renderAt(initialUrl: string, showSalesRegionFilter = false) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialUrl]}>
        <Routes>
          <Route
            path="/cases"
            element={
              <>
                <CsmIssuesView title="Cases" showSalesRegionFilter={showSalesRegionFilter} />
                <LocationSearchProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}


/** The filters object `useGetCsmCases` was last called with. */
function lastQueryFilters(): { salesRegions: string[] } {
  const calls = searchSpy.mock.calls;
  return calls[calls.length - 1][0] as { salesRegions: string[] };
}

describe("CsmIssuesView — Sales Region is scoped to views that opt in", () => {
  it("drops a salesRegions URL value from the query on a view without the control", () => {
    renderAt("/cases?salesRegions=EMEA");

    expect(lastQueryFilters().salesRegions).toEqual([]);
  });

  it("sends the salesRegions URL value on a view that shows the control", () => {
    renderAt("/cases?salesRegions=EMEA", true);

    expect(lastQueryFilters().salesRegions).toEqual(["EMEA"]);
  });
});
