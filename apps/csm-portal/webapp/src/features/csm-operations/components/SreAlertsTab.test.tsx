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

import type { JSX } from "react";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import SreAlertsTab from "@features/csm-operations/components/SreAlertsTab";
import { useSearchSreAlerts, useSreAlertFacets } from "@features/csm-operations/api/useSreAlerts";
import type { BeSreAlert, BeSreAlertSearchPayload } from "@api/backend/types";

// The backend client reads runtime config at module load, which isn't present under vitest.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn(), get: vi.fn() }),
}));
vi.mock("@features/csm-operations/api/useSreAlerts", () => ({
  useSearchSreAlerts: vi.fn(),
  useSreAlertFacets: vi.fn(),
}));
// Only the filter bar's open/closed storage key derives from the signed-in user.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

const mockedSearch = vi.mocked(useSearchSreAlerts);
const mockedFacets = vi.mocked(useSreAlertFacets);

const LINKED: BeSreAlert = {
  id: "01a21bb0-c1a4-7954-8ee2-f7cb2d4b63f7",
  number: "ALT000000042",
  source: "Azure Monitor",
  service: "svc-a",
  environment: "prod",
  metricName: "cpu",
  severity: "critical",
  category: "Infrastructure",
  tier: "priority",
  receivedOn: "2026-11-27T10:00:00Z",
  createdOn: "2026-11-27T10:00:01Z",
  incident: {
    id: "11111111-1111-1111-1111-111111111111",
    number: "INC0012345",
    subject: "Gateway CPU saturated",
    state: "IN_PROGRESS",
    priority: "HIGH",
    impact: "HIGH",
    urgency: "MEDIUM",
    openedOn: "2026-11-27T10:05:00Z",
  },
};

const UNLINKED: BeSreAlert = {
  ...LINKED,
  id: "02b32cc1-d2b5-4a65-9ff3-a8dc3e5c74a8",
  number: "ALT000000043",
  severity: "warning",
  tier: "standard",
  metricName: "",
  incident: null,
};

function mockSearch(overrides: Partial<ReturnType<typeof useSearchSreAlerts>> = {}): void {
  mockedSearch.mockReturnValue({
    data: { alerts: [LINKED, UNLINKED], total: 2, offset: 0, limit: 20 },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchSreAlerts>);
}

function lastPayload(): BeSreAlertSearchPayload {
  return mockedSearch.mock.calls[mockedSearch.mock.calls.length - 1][0];
}

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return (
    <div data-testid="location">
      {location.pathname}
      {location.search}
      {JSON.stringify(location.state)}
    </div>
  );
}

function renderTab(initial = "/operations/alerts"): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initial]}>
        <Routes>
          <Route
            path="/operations/alerts"
            element={
              <>
                <SreAlertsTab />
                <LocationProbe />
              </>
            }
          />
          <Route path="/operations/alerts/:id" element={<LocationProbe />} />
          <Route path="/operations/incidents/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mockedSearch.mockReset();
  mockedFacets.mockReset();
  mockedFacets.mockReturnValue({
    data: { environments: ["prod", "staging"], services: ["svc-a"], severities: ["critical", "warning"], sources: ["Azure Monitor"] },
  } as unknown as ReturnType<typeof useSreAlertFacets>);
  window.localStorage?.clear();
});

describe("SreAlertsTab", () => {
  it("renders each alert's number, severity, tier, and incident", () => {
    mockSearch();
    renderTab();

    for (const header of ["Number", "Received", "Severity", "Service", "Environment", "Metric", "Source", "Tier", "Incident"]) {
      expect(screen.getByRole("columnheader", { name: header })).toBeInTheDocument();
    }
    const linkedRow = screen.getByRole("row", { name: /View alert ALT000000042/ });
    expect(within(linkedRow).getByText("ALT000000042")).toBeInTheDocument();
    expect(within(linkedRow).getByText("Critical")).toBeInTheDocument();
    expect(within(linkedRow).getByText("Priority")).toBeInTheDocument();
    expect(within(linkedRow).getByRole("link", { name: "INC0012345" })).toHaveAttribute(
      "href",
      "/operations/incidents/11111111-1111-1111-1111-111111111111",
    );

    // The linked incident's state, as the incident list colours it.
    expect(within(linkedRow).getByText("In Progress")).toBeInTheDocument();

    const unlinkedRow = screen.getByRole("row", { name: /View alert ALT000000043/ });
    expect(within(unlinkedRow).getByText("Warning")).toBeInTheDocument();
    expect(within(unlinkedRow).getByText("Standard")).toBeInTheDocument();
    // Its only link is its own number: there is no incident to open.
    expect(within(unlinkedRow).getAllByRole("link").map((l) => l.textContent)).toEqual(["ALT000000043"]);
    // No metric and no incident each show a dash.
    expect(within(unlinkedRow).getAllByText("—")).toHaveLength(2);
  });

  it("reads a CANCELED incident state as the incident pages' Cancelled", () => {
    mockSearch({
      data: {
        alerts: [{ ...LINKED, incident: { ...LINKED.incident!, state: "CANCELED" } }],
        total: 1,
        offset: 0,
        limit: 20,
      },
    });
    renderTab();
    expect(screen.getByText("Cancelled")).toBeInTheDocument();
  });

  it("colours the severity chip by severity", () => {
    mockSearch();
    renderTab();
    const critical = within(screen.getByRole("row", { name: /ALT000000042/ })).getByText("Critical").closest(".MuiChip-root");
    const warning = within(screen.getByRole("row", { name: /ALT000000043/ })).getByText("Warning").closest(".MuiChip-root");
    expect(critical?.className).toMatch(/colorError/);
    expect(warning?.className).toMatch(/colorWarning/);
  });

  it("asks for the newest received first, 20 per page", () => {
    mockSearch();
    renderTab();
    expect(lastPayload()).toEqual({
      filters: {},
      sortBy: { field: "receivedOn", order: "desc" },
      pagination: { offset: 0, limit: 20 },
    });
  });

  it("opens the alert by its UUID with the list as its Back target", () => {
    mockSearch();
    renderTab("/operations/alerts?altTiers=priority");
    fireEvent.click(screen.getByRole("row", { name: /View alert ALT000000042/ }));
    const location = screen.getByTestId("location").textContent ?? "";
    expect(location).toContain(`/operations/alerts/${LINKED.id}`);
    expect(location).toContain('"from":"/operations/alerts?altTiers=priority"');
  });

  it("opens the alert from its number, a link the keyboard can reach", () => {
    mockSearch();
    renderTab("/operations/alerts?altTiers=priority");
    const link = screen.getByRole("link", { name: "ALT000000042" });
    expect(link).toHaveAttribute("href", `/operations/alerts/${LINKED.id}`);
    fireEvent.click(link);
    const location = screen.getByTestId("location").textContent ?? "";
    expect(location).toContain(`/operations/alerts/${LINKED.id}`);
    expect(location).toContain('"from":"/operations/alerts?altTiers=priority"');
  });

  it("shows a dash for a severity the source didn't send", () => {
    mockSearch({ data: { alerts: [{ ...UNLINKED, severity: null, category: null }], total: 1, offset: 0, limit: 20 } });
    renderTab();
    const row = screen.getByRole("row", { name: /View alert ALT000000043/ });
    expect(within(row).getAllByText("—")).toHaveLength(3);
  });

  it("opens the linked incident, not the alert, from the incident link", () => {
    mockSearch();
    renderTab();
    fireEvent.click(screen.getByRole("link", { name: "INC0012345" }));
    expect(screen.getByTestId("location").textContent).toContain(
      "/operations/incidents/11111111-1111-1111-1111-111111111111",
    );
  });

  it("reads the filters from the URL into the search", () => {
    mockSearch();
    renderTab(
      "/operations/alerts?altEnvironments=prod&altServices=svc-a&altSeverities=critical&altSources=Azure%20Monitor" +
        "&altTiers=priority&altTiers=bogus&altIncident=unlinked&altReceivedFrom=2026-11-01&altReceivedTo=2026-11-26&altQ=cpu",
    );
    expect(screen.getByRole("button", { name: "Filters (7)" })).toBeInTheDocument();
    return waitFor(() =>
      expect(lastPayload().filters).toEqual({
        searchQuery: "cpu",
        environments: ["prod"],
        services: ["svc-a"],
        severities: ["critical"],
        sources: ["Azure Monitor"],
        tiers: ["priority"],
        incident: "unlinked",
        receivedFrom: "2026-11-01T00:00:00Z",
        // The last day is included: the backend's receivedTo is exclusive.
        receivedTo: "2026-11-27T00:00:00Z",
      }),
    );
  });

  it("writes a filter change to the URL and clears it again", async () => {
    mockSearch();
    renderTab();
    fireEvent.mouseDown(screen.getByLabelText("Incident"));
    fireEvent.click(await screen.findByRole("option", { name: "Linked to an incident" }));
    await waitFor(() => expect(screen.getByTestId("location").textContent).toContain("altIncident=linked"));
    expect(lastPayload().filters).toEqual({ incident: "linked" });

    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    await waitFor(() => expect(screen.getByTestId("location").textContent).not.toContain("altIncident"));
  });

  it("keeps the state the list was opened with when a filter changes", async () => {
    mockSearch();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[{ pathname: "/operations/alerts", state: { from: "/dashboard" } }]}>
          <Routes>
            <Route
              path="/operations/alerts"
              element={
                <>
                  <SreAlertsTab />
                  <LocationProbe />
                </>
              }
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.mouseDown(screen.getByLabelText("Incident"));
    fireEvent.click(await screen.findByRole("option", { name: "Linked to an incident" }));
    await waitFor(() => expect(screen.getByTestId("location").textContent).toContain("altIncident=linked"));
    expect(screen.getByTestId("location").textContent).toContain('{"from":"/dashboard"}');
  });

  it("searches after the user stops typing", async () => {
    mockSearch();
    renderTab();
    fireEvent.change(screen.getByRole("textbox", { name: "Search alerts" }), { target: { value: "  ALT000000042 " } });
    expect(screen.getByTestId("location").textContent).toContain("altQ=");
    await waitFor(() => expect(lastPayload().filters).toEqual({ searchQuery: "ALT000000042" }));
  });

  it("sorts by a header and flips the order on a second click", () => {
    mockSearch();
    renderTab();
    fireEvent.click(within(screen.getByRole("columnheader", { name: "Number" })).getByRole("button"));
    expect(lastPayload().sortBy).toEqual({ field: "number", order: "desc" });
    fireEvent.click(within(screen.getByRole("columnheader", { name: "Number" })).getByRole("button"));
    expect(lastPayload().sortBy).toEqual({ field: "number", order: "asc" });
    // Metric isn't sortable on the backend.
    expect(within(screen.getByRole("columnheader", { name: "Metric" })).queryByRole("button")).not.toBeInTheDocument();
  });

  it("pages through the results", () => {
    mockSearch({ data: { alerts: [LINKED], total: 45, offset: 0, limit: 20 } });
    renderTab();
    fireEvent.click(screen.getByRole("button", { name: "Go to next page" }));
    expect(lastPayload().pagination).toEqual({ offset: 20, limit: 20 });
  });

  it("shows the empty state", () => {
    mockSearch({ data: { alerts: [], total: 0, offset: 0, limit: 20 } });
    renderTab();
    expect(screen.getByText("No alerts found.")).toBeInTheDocument();
  });

  it("shows the error state", () => {
    mockSearch({ data: undefined, isError: true, error: new Error("Failed to search alerts.") });
    renderTab();
    expect(screen.getByText("Failed to search alerts.")).toBeInTheDocument();
  });
});
