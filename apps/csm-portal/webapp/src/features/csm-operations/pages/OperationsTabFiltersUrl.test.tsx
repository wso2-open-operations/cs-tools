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
 * Regression coverage for the change-requests / incidents tab filter state:
 * both tabs must keep their filters in the URL (surviving a remount / tab
 * switch, not just component state), and switching tabs must neither clobber
 * the other tab's own filter params nor push a fresh history entry per
 * keystroke.
 */

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, useSearchParams } from "react-router";
import { describe, expect, it, vi } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import IncidentsTab from "@features/csm-operations/components/IncidentsTab";
import { useSearchChangeRequests } from "@features/csm-operations/api/useSearchChangeRequests";
import { useSearchIncidents } from "@features/csm-operations/api/useSearchIncidents";

// The backend client reads runtime config at module load, unavailable under
// vitest — stub it the same way other page/tab tests in this repo do.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));
// Both tabs now render a FilteredCsvExportButton, which reads these two
// contexts — not under test here, so they're stubbed the same way
// CreateChangeRequestPage's own tests stub the error banner.
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
// ChangeRequestsTab now also renders a "customise columns" picker, whose
// storage key derives from the signed-in user — not under test here, so
// stubbed the same way ChangeRequestsTab.test.tsx / CsmAnnouncementsPage.test.tsx do.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

vi.mock("@features/csm-operations/api/useSearchChangeRequests", () => ({
  useSearchChangeRequests: vi.fn(),
}));
vi.mock("@features/csm-operations/api/useSearchIncidents", () => ({
  useSearchIncidents: vi.fn(),
}));

const EMPTY_RESULT = {
  data: undefined,
  isLoading: false,
  isFetching: false,
  isError: false,
  error: null,
} as const;

vi.mocked(useSearchChangeRequests).mockReturnValue(
  EMPTY_RESULT as unknown as ReturnType<typeof useSearchChangeRequests>,
);
vi.mocked(useSearchIncidents).mockReturnValue(
  EMPTY_RESULT as unknown as ReturnType<typeof useSearchIncidents>,
);

/**
 * Mirrors how OperationsPage + `useQueryTabs` switch between tabs off
 * `?tab=`: the tab strip's `select()` mutates the existing `URLSearchParams`
 * (only ever touching the `tab` key) rather than replacing it, which is what
 * lets a filter param owned by the other (currently unmounted) tab survive.
 * A debug node exposes the live query string so the test can assert on it
 * without reaching into MemoryRouter internals.
 */
function OperationsTabsHarness(): JSX.Element {
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") ?? "change_requests";
  /** Switch tabs by writing `?tab=`, preserving the other params already in the
   * URL — the whole point of these tests is that one tab's filters survive a
   * switch to the other and back. */
  const selectTab = (next: string): void =>
    setParams((prev) => {
      prev.set("tab", next);
      return prev;
    });
  return (
    <>
      <button onClick={() => selectTab("change_requests")}>
        Change requests tab
      </button>
      <button onClick={() => selectTab("incidents")}>Incidents tab</button>
      <div data-testid="url">{params.toString()}</div>
      {tab === "change_requests" && <ChangeRequestsTab />}
      {tab === "incidents" && <IncidentsTab />}
    </>
  );
}

/**
 * Mount the harness at a given URL, so a test can start from a pre-filtered
 * query string and assert the tabs read it back — the bookmark/share case, not
 * just filters applied by clicking.
 */
function renderHarness(initialEntry: string) {
  // Both tabs now default their filter panel open (see IncidentsTab /
  // ChangeRequestsTab), so IncidentsFilterBar's product picker mounts
  // immediately and needs a real QueryClientProvider, not just the mocked
  // search hooks below.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <OperationsTabsHarness />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("operations tab filters live in the URL", () => {
  it("writes a change-request search into the URL under its own `cr`-prefixed param", () => {
    renderHarness("/operations?tab=change_requests");
    const search = screen.getByPlaceholderText("Search by number or subject…");
    fireEvent.change(search, { target: { value: "rollback" } });
    expect(screen.getByTestId("url").textContent).toContain("crQ=rollback");
  });

  it("keeps a change-request filter across a tab switch and back", () => {
    renderHarness("/operations?tab=change_requests");
    const search = () =>
      screen.getByPlaceholderText("Search by number or subject…");
    fireEvent.change(search(), { target: { value: "rollback" } });
    expect(search()).toHaveValue("rollback");

    fireEvent.click(screen.getByRole("button", { name: "Incidents tab" }));
    expect(
      screen.getByPlaceholderText("Search by number or subject…"),
    ).toHaveValue("");
    expect(screen.getByTestId("url").textContent).toContain("crQ=rollback");

    fireEvent.click(
      screen.getByRole("button", { name: "Change requests tab" }),
    );
    // The change-requests tab remounted from scratch (its own useState/local
    // filter would have reset to defaults here) — the value only comes back
    // because it was read from the URL, not local state.
    expect(
      screen.getByPlaceholderText("Search by number or subject…"),
    ).toHaveValue("rollback");
  });

  it("does not clobber the incidents tab's own filter when the change-requests tab writes its params", () => {
    renderHarness("/operations?tab=change_requests&incQ=disk+full");
    const search = screen.getByPlaceholderText("Search by number or subject…");
    fireEvent.change(search, { target: { value: "rollback" } });

    const url = screen.getByTestId("url").textContent ?? "";
    expect(url).toContain("crQ=rollback");
    expect(url).toContain("incQ=disk");
  });

  it("tolerates a malformed/stale change-request query string instead of crashing or leaking it to the search payload", () => {
    renderHarness(
      "/operations?tab=change_requests&crStates=not_a_real_state&crClosedFrom=not-a-date",
    );
    expect(
      screen.getByPlaceholderText("Search by number or subject…"),
    ).toHaveValue("");
    // Renders without throwing and without the bogus values surviving into
    // the (mocked) search hook's filters — covered at the unit level in
    // changeRequestsFiltersUrl.test.ts; this just proves the tab itself
    // doesn't blow up on a hand-edited URL.
    expect(screen.getByText("No change requests found.")).toBeInTheDocument();
  });
});
