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
import SreAlertDetailPage from "@features/csm-operations/pages/SreAlertDetailPage";
import { useGetSreAlert } from "@features/csm-operations/api/useSreAlerts";
import type { BeSreAlertDetail } from "@api/backend/types";

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn(), get: vi.fn() }),
}));
vi.mock("@features/csm-operations/api/useSreAlerts", () => ({
  useGetSreAlert: vi.fn(),
}));

const mockedGet = vi.mocked(useGetSreAlert);

const ALERT: BeSreAlertDetail = {
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
  uniqueIdentifier: "alarm-7",
  description: "CPU above 90% for 5 minutes",
  assignmentGroup: "",
  rawBody: '{"alarm":"cpu","value":93}',
};

function mockAlert(overrides: Partial<ReturnType<typeof useGetSreAlert>>): void {
  mockedGet.mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  } as unknown as ReturnType<typeof useGetSreAlert>);
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

function renderPage(entry: { pathname: string; state?: unknown } = { pathname: `/operations/alerts/${ALERT.id}` }): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/operations/alerts/:id" element={<SreAlertDetailPage />} />
          <Route path="/operations/alerts" element={<LocationProbe />} />
          <Route path="/operations/incidents/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const writeText = vi.fn();

beforeEach(() => {
  mockedGet.mockReset();
  writeText.mockReset();
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
});

describe("SreAlertDetailPage", () => {
  it("looks the alert up by the id in the path", () => {
    mockAlert({ data: ALERT });
    renderPage({ pathname: "/operations/alerts/ALT000000042" });
    expect(mockedGet).toHaveBeenCalledWith("ALT000000042");
  });

  it("shows the ALT number, the UUID, and the severity and tier chips in the header", () => {
    mockAlert({ data: ALERT });
    renderPage();
    expect(screen.getByRole("heading", { name: "ALT000000042" })).toBeInTheDocument();
    expect(screen.getByTestId("sre-alert-uuid")).toHaveTextContent(ALERT.id);
    expect(screen.getByText("Critical").closest(".MuiChip-root")?.className).toMatch(/colorError/);
    expect(screen.getByText("Priority tier")).toBeInTheDocument();
  });

  it("copies the UUID", async () => {
    writeText.mockResolvedValue(undefined);
    mockAlert({ data: ALERT });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Copy alert ID" }));
    expect(writeText).toHaveBeenCalledWith(ALERT.id);
    // The copy icon turns into a check mark once the browser has it.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Copy alert ID" }).querySelector(".lucide-check")).not.toBeNull(),
    );
  });

  it("lays the alert out like an incident: what fired as the heading, then its description", () => {
    mockAlert({ data: ALERT });
    renderPage();
    expect(screen.getByRole("heading", { name: "cpu on svc-a" })).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Description" })).getByText("CPU above 90% for 5 minutes")).toBeInTheDocument();
  });

  it("leaves the description card out when the source sent none", () => {
    mockAlert({ data: { ...ALERT, description: null } });
    renderPage();
    expect(screen.queryByRole("region", { name: "Description" })).not.toBeInTheDocument();
  });

  it("shows every field in the Overview, with a dash for an empty value", () => {
    mockAlert({ data: ALERT });
    renderPage();
    const overview = within(screen.getByRole("region", { name: "Overview" }));
    expect(overview.getByText("Azure Monitor")).toBeInTheDocument();
    expect(overview.getByText("svc-a")).toBeInTheDocument();
    expect(overview.getByText("alarm-7")).toBeInTheDocument();
    expect(overview.getByText("Received on")).toBeInTheDocument();
    expect(overview.getByText("Created on")).toBeInTheDocument();
    // The assignment group is empty.
    expect(overview.getByText("Assignment group").nextElementSibling).toHaveTextContent("—");
    // The gateway route, source topic and account, and the queue message id and index are not stored, so not shown.
    expect(screen.queryByText("Route")).not.toBeInTheDocument();
    expect(screen.queryByText("Message ID")).not.toBeInTheDocument();
  });

  it("shows the linked incident in the Overview like the incident page's runbook", () => {
    mockAlert({ data: ALERT });
    renderPage();
    const section = within(screen.getByRole("region", { name: "Incident" }));
    expect(section.getByText("Gateway CPU saturated (INC0012345)")).toBeInTheDocument();
    expect(section.getByText("In Progress")).toBeInTheDocument();
    // The incident page's own P1-P5 badge.
    expect(section.getByText("High (P2)")).toBeInTheDocument();
    expect(section.getByText(/^Impact High · Urgency Medium · Opened /)).not.toHaveTextContent("Opened —");
    expect(section.getByRole("link", { name: "Open incident" })).toHaveAttribute(
      "href",
      "/operations/incidents/11111111-1111-1111-1111-111111111111",
    );
  });

  it("shows a dash for an incident with no opened date", () => {
    mockAlert({ data: { ...ALERT, incident: { ...ALERT.incident!, openedOn: null, impact: "" } } });
    renderPage();
    const section = within(screen.getByRole("region", { name: "Incident" }));
    expect(section.getByText("Impact — · Urgency Medium · Opened —")).toBeInTheDocument();
  });

  it("opens the incident, with this alert as the incident's Back target", () => {
    mockAlert({ data: ALERT });
    renderPage({ pathname: `/operations/alerts/${ALERT.id}`, state: { from: "/operations/alerts?altTiers=priority" } });
    fireEvent.click(within(screen.getByRole("region", { name: "Incident" })).getByRole("link", { name: "Open incident" }));
    const location = screen.getByTestId("location").textContent ?? "";
    expect(location).toContain("/operations/incidents/11111111-1111-1111-1111-111111111111");
    expect(location).toContain(`"from":"/operations/alerts/${ALERT.id}"`);
    expect(location).toContain('"parentState":{"from":"/operations/alerts?altTiers=priority"}');
  });

  it("says when the alert isn't linked to an incident", () => {
    mockAlert({ data: { ...ALERT, incident: null } });
    renderPage();
    const section = within(screen.getByRole("region", { name: "Incident" }));
    expect(section.getByText("This alert is not linked to an incident.")).toBeInTheDocument();
    expect(section.queryByRole("link")).not.toBeInTheDocument();
  });

  it("pretty-prints a JSON payload", () => {
    mockAlert({ data: ALERT });
    renderPage();
    expect(screen.getByTestId("sre-alert-raw-payload").textContent).toBe('{\n  "alarm": "cpu",\n  "value": 93\n}');
  });

  it("keeps every value of a JSON payload as the source sent it", () => {
    mockAlert({ data: { ...ALERT, rawBody: '{"eventId":9007199254740993,"ratio":1.50,"tags":[],"note":"a, b: {c}"}' } });
    renderPage();
    expect(screen.getByTestId("sre-alert-raw-payload").textContent).toBe(
      '{\n  "eventId": 9007199254740993,\n  "ratio": 1.50,\n  "tags": [],\n  "note": "a, b: {c}"\n}',
    );
  });

  it("shows a payload that isn't JSON as it came", () => {
    mockAlert({ data: { ...ALERT, rawBody: "alarm=cpu value=93" } });
    renderPage();
    expect(screen.getByTestId("sre-alert-raw-payload").textContent).toBe("alarm=cpu value=93");
  });

  it("explains a missing payload", () => {
    mockAlert({ data: { ...ALERT, rawBody: null } });
    renderPage();
    expect(screen.queryByTestId("sre-alert-raw-payload")).not.toBeInTheDocument();
    expect(screen.getByText("Only the first alert of a message keeps its payload.")).toBeInTheDocument();
  });

  it("goes Back to the filtered list it came from, with that page's state", async () => {
    mockAlert({ data: ALERT });
    renderPage({
      pathname: `/operations/alerts/${ALERT.id}`,
      state: { from: "/operations/alerts?altTiers=priority", parentState: { from: "/dashboard" } },
    });
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    await waitFor(() =>
      expect(screen.getByTestId("location").textContent).toBe('/operations/alerts?altTiers=priority{"from":"/dashboard"}'),
    );
  });

  it("goes Back to the Alerts tab when opened directly", async () => {
    mockAlert({ data: ALERT });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    await waitFor(() => expect(screen.getByTestId("location").textContent).toContain("/operations/alerts"));
  });

  it("says when the alert doesn't exist", () => {
    mockAlert({ data: null });
    renderPage();
    expect(screen.getByText("Alert not found.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
  });

  it("shows the error state", () => {
    mockAlert({ isError: true, error: new Error("Failed to retrieve alert.") });
    renderPage();
    expect(screen.getByText("Failed to retrieve alert.")).toBeInTheDocument();
  });
});
