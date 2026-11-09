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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import type { JSX, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

import IncidentTasksWidget from "@features/csm-operations/components/IncidentTasksWidget";
import { useSearchIncidentTasks } from "@features/csm-operations/api/useSearchIncidentTasks";

const INCIDENT_ID = "11111111-1111-4111-8111-111111111111";

let queryClient: QueryClient;
beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

const wrapper = ({ children }: { children: ReactNode }): JSX.Element => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

describe("useSearchIncidentTasks", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("asks for this incident's tasks with the generic incidentId filter", async () => {
    postMock.mockResolvedValue({ incidentTasks: [], total: 0, limit: 50, offset: 0 });
    const { result } = renderHook(() => useSearchIncidentTasks(INCIDENT_ID), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith("/incident-tasks/search", {
      filters: { filters: [{ field: "incidentId", op: "in", values: [INCIDENT_ID] }] },
      pagination: { offset: 0, limit: 50 },
    });
  });
});

// A fresh client per render, as LinkedIncidentsListWidget.test.tsx does.
function renderWidget(locationState?: unknown): void {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={[{ pathname: `/operations/incidents/${INCIDENT_ID}`, state: locationState }]}>
        <IncidentTasksWidget incidentId={INCIDENT_ID} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("IncidentTasksWidget", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("lists the report and alert tasks with state, group and assignee", async () => {
    postMock.mockResolvedValue({
      incidentTasks: [
        {
          id: "t1",
          number: "CS-PORTAL-000021",
          subject: "[Incident Report] Create the incident report for INC0099782",
          stateLabel: "Open",
          assignmentGroup: { id: "g1", name: "Choreo SRE Team" },
          assignedTo: { id: "u1", name: "Jane Doe" },
        },
        {
          id: "t2",
          number: "CS-PORTAL-000022",
          subject: "[Alert Task][Falser Alarm] INC0099782 alert is a false alarm",
          stateLabel: "Open",
          assignmentGroup: { id: "g2", name: "WSO2 SRE Team" },
        },
      ],
      total: 2,
      limit: 50,
      offset: 0,
    });
    renderWidget();

    expect(await screen.findByText("Incident tasks (2)")).toBeInTheDocument();
    expect(
      screen.getByText("CS-PORTAL-000022 — [Alert Task][Falser Alarm] INC0099782 alert is a false alarm"),
    ).toBeInTheDocument();
    expect(screen.getByText("WSO2 SRE Team")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("opens a task's detail page, with Back returning to the Related tab", async () => {
    navigateMock.mockReset();
    postMock.mockResolvedValue({
      incidentTasks: [{ id: "t1", number: "CS-PORTAL-000021", subject: "[Incident Report] Write it", stateLabel: "Open" }],
      total: 1,
      limit: 50,
      offset: 0,
    });
    renderWidget();

    const link = await screen.findByRole("link", { name: "CS-PORTAL-000021 — [Incident Report] Write it" });
    expect(link).toHaveAttribute("href", "/operations/incident-tasks/t1");

    fireEvent.click(screen.getByText("Open"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/incident-tasks/t1", {
      state: { from: `/operations/incidents/${INCIDENT_ID}?tab=related`, parentState: null },
    });
  });

  it("passes the incident page's own state on as parentState, so the incident keeps its Back", async () => {
    navigateMock.mockReset();
    postMock.mockResolvedValue({
      incidentTasks: [{ id: "t1", number: "CS-PORTAL-000021", subject: "[Incident Report] Write it", stateLabel: "Open" }],
      total: 1,
      limit: 50,
      offset: 0,
    });
    const incidentState = { from: "/operations?tab=incidents", parentState: { from: "/dashboard/system/7" } };
    renderWidget(incidentState);

    fireEvent.click(await screen.findByText("Open"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/incident-tasks/t1", {
      state: { from: `/operations/incidents/${INCIDENT_ID}?tab=related`, parentState: incidentState },
    });
  });

  it("says so when the incident has no tasks", async () => {
    postMock.mockResolvedValue({ incidentTasks: [], total: 0, limit: 50, offset: 0 });
    renderWidget();
    expect(await screen.findByText("No tasks for this incident.")).toBeInTheDocument();
  });

  it("reports a failed load instead of an empty list", async () => {
    postMock.mockRejectedValue(new Error("boom"));
    renderWidget();
    await waitFor(() =>
      expect(screen.getByText("Could not load the tasks for this incident.")).toBeInTheDocument(),
    );
  });
});
