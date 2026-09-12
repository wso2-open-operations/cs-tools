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

describe("CsmIssuesView + real CasesFilterBar — work state clears fully from the URL", () => {
  // Regression test for: with only "In progress" selected, checking then
  // unchecking a work state left the last-checked value permanently stuck
  // (could only toggle between work states, never reach none). Root cause:
  // CsmIssuesView's FILTER_PARAM_KEYS (the URL keys it deletes before
  // rewriting from the next filter state) didn't include "workStates", so an
  // empty next.workStates never actually cleared the stale URL param, and
  // the next render read the stale value straight back in.
  //
  // `workStates` no longer has its own bar control (see CasesFilterBar's
  // "CRE Team" control, which replaced it) -- the State control's own onChange
  // is what clears a stale `workStates` now, when the selection widens past
  // "work_in_progress" alone. This exercises that same URL round trip
  // (not just the in-memory filter object CasesFilterBar.test.tsx already
  // covers) via the "Work state: Ongoing" chip that renders instead.
  it("adding a second state clears the stale workStates chip and its URL param, not just the in-memory value", () => {
    renderAt("/cases?states=work_in_progress&workStates=ongoing");

    expect(screen.getByText("Work state: Ongoing")).toBeInTheDocument();

    // Open the State dropdown and add a second state alongside
    // work_in_progress -- CasesFilterBar's own onChange clears workStates
    // as soon as the selection stops being exactly that one state.
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: "Open" }));

    // The chip -- and so the URL param behind it -- must be gone, not just
    // hidden while the stale value lingers underneath.
    expect(screen.queryByText("Work state: Ongoing")).not.toBeInTheDocument();
    expect(screen.getByTestId("search-probe").textContent).not.toContain("workStates");
  });
});

describe("CsmIssuesView + real CasesFilterBar — onboarding status clears fully from the URL", () => {
  // Same root cause as the workStates regression above, found live:
  // FILTER_PARAM_KEYS was missing "excludeStates" -- a leftover from when
  // this control briefly stored its selection as an exclude-flavored field.
  // The control now edits the real `onboardingStatuses` field directly
  // (already covered by FILTER_PARAM_KEYS), so this exercises the same URL
  // round trip via that field's own real bar control rather than a separate
  // exclude one.
  it("unchecking the only selected onboarding status clears it from the URL, not just in memory", () => {
    renderAt("/cases?onboardingStatuses=Completed");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    fireEvent.click(screen.getByRole("option", { name: "Completed" }));

    expect(screen.getByTestId("search-probe").textContent).not.toContain(
      "onboardingStatuses",
    );
  });
});
