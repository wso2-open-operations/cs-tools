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

/**
 * Regression coverage for the problems tab's filter state now living in the
 * URL (`prob...`-prefixed params) instead of local component state — same
 * pattern the change-requests/incidents tabs already have their own coverage
 * for (`OperationsTabFiltersUrl.test.tsx`), plus a smoke test for the new
 * "Saved views" menu this tab now shares with them.
 */

import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, useSearchParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import ProblemsTab from "@features/csm-operations/components/ProblemsTab";
import { useSearchProblems } from "@features/csm-operations/api/useSearchProblems";

const savedViewsState = vi.hoisted(() => ({
  views: [] as { name: string; qs: string }[],
}));

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));
vi.mock("@features/saved-filter-views/useSavedFilterViews", () => ({
  useSavedFilterViews: () => ({
    views: savedViewsState.views,
    isLoading: false,
    saveFilterView: vi.fn(),
    deleteFilterView: vi.fn(),
    moveFilterView: vi.fn(),
  }),
}));

vi.mock("@features/csm-operations/api/useSearchProblems", () => ({
  useSearchProblems: vi.fn(),
}));

// The filter bar's SRE Team control goes through the shared team registry
// query — stub it out, same as ChangeRequestsTab.test.tsx's own mock.
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => ({ data: [], isLoading: false }),
}));

// Only the filter-section collapsed state's storage key derives from the
// signed-in user (same mocks as IncidentsTab.test.tsx/ChangeRequestsTab.test.tsx).
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

const mockedUseSearch = vi.mocked(useSearchProblems);

function mockResult(overrides: Partial<ReturnType<typeof useSearchProblems>>): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchProblems>);
}

/** Exposes the live query string alongside the tab, so a test can assert on
 * it without reaching into `MemoryRouter` internals — mirrors
 * `OperationsTabFiltersUrl.test.tsx`'s own harness. */
function ProblemsTabHarness(): JSX.Element {
  const [params] = useSearchParams();
  return (
    <>
      <div data-testid="url">{params.toString()}</div>
      <ProblemsTab />
    </>
  );
}

function renderTab(initialEntry = "/operations?tab=problems") {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <ProblemsTabHarness />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  window.localStorage?.clear();
  savedViewsState.views = [];
  mockResult({});
});

describe("ProblemsTab — filters live in the URL", () => {
  it("writes a search into the URL under its own `prob`-prefixed param", () => {
    renderTab();
    const search = screen.getByPlaceholderText("Search by number or subject…");
    fireEvent.change(search, { target: { value: "outage" } });
    expect(screen.getByTestId("url").textContent).toContain("probQ=outage");
  });

  it("reads an already-filtered query string back on mount (bookmark/share case)", () => {
    renderTab("/operations?tab=problems&probQ=outage&probStates=NEW");
    expect(
      screen.getByPlaceholderText("Search by number or subject…"),
    ).toHaveValue("outage");
    expect(screen.getByRole("button", { name: /filters \(1\)/i })).toBeInTheDocument();
  });

  it("tolerates a malformed/stale query string instead of crashing", () => {
    renderTab("/operations?tab=problems&probStates=not_a_real_state");
    expect(
      screen.getByPlaceholderText("Search by number or subject…"),
    ).toHaveValue("");
  });
});

describe("ProblemsTab — Saved views", () => {
  it("renders the shared Saved views menu, scoped to this tab's own store", () => {
    renderTab();
    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    expect(screen.getByText(/no saved views yet/i)).toBeInTheDocument();
  });

  it("applying a saved problems view updates the URL", () => {
    savedViewsState.views = [{ name: "New problems", qs: "probStates=NEW" }];
    renderTab();

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByText("New problems"));

    expect(screen.getByTestId("url").textContent).toContain("probStates=NEW");
  });
});
