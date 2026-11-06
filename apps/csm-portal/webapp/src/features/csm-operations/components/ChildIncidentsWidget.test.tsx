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

import ChildIncidentsWidget from "@features/csm-operations/components/ChildIncidentsWidget";
import { useSearchChildIncidents } from "@features/csm-operations/api/useSearchChildIncidents";

const PARENT_ID = "11111111-1111-4111-8111-111111111111";

let queryClient: QueryClient;
beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  postMock.mockReset();
  navigateMock.mockReset();
});

const wrapper = ({ children }: { children: ReactNode }): JSX.Element => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

function incidentRow(n: number): Record<string, unknown> {
  return { id: `inc-${n}`, number: `INC${String(n).padStart(7, "0")}`, subject: `Child ${n}`, state: "NEW" };
}

describe("useSearchChildIncidents", () => {
  it("asks for this incident's children by parentIncidentIds, not the generic parentIds", async () => {
    postMock.mockResolvedValue({ incidents: [], total: 0, limit: 50, offset: 0 });
    const { result } = renderHook(() => useSearchChildIncidents(PARENT_ID), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith("/incidents/search", {
      filters: { parentIncidentIds: [PARENT_ID] },
      pagination: { offset: 0, limit: 50 },
    });
  });

  it("pages through every child, not just the first page", async () => {
    const all = Array.from({ length: 120 }, (_, i) => incidentRow(i + 1));
    postMock.mockImplementation((_url: string, body: { pagination: { offset: number; limit: number } }) => {
      const { offset, limit } = body.pagination;
      return Promise.resolve({ incidents: all.slice(offset, offset + limit), total: all.length, limit, offset });
    });
    const { result } = renderHook(() => useSearchChildIncidents(PARENT_ID), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(3);
    expect(postMock.mock.calls.map((c) => c[1].pagination.offset)).toEqual([0, 50, 100]);
    expect(result.current.data?.incidents).toHaveLength(120);
    expect(result.current.data?.total).toBe(120);
  });

  it("stops at the reported total when the last page is exactly full", async () => {
    const all = Array.from({ length: 50 }, (_, i) => incidentRow(i + 1));
    postMock.mockResolvedValue({ incidents: all, total: 50, limit: 50, offset: 0 });
    const { result } = renderHook(() => useSearchChildIncidents(PARENT_ID), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("is keyed under the incidents list, so an incident PATCH's list refresh covers it", async () => {
    postMock.mockResolvedValue({ incidents: [], total: 0, limit: 50, offset: 0 });
    const { result } = renderHook(() => useSearchChildIncidents(PARENT_ID), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(queryClient.getQueryCache().findAll({ queryKey: ["incidents"] })).toHaveLength(1);
  });

  it("sends nothing until it has an incident id", () => {
    renderHook(() => useSearchChildIncidents(undefined), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });
});

function renderWidget(): void {
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ChildIncidentsWidget parentIncidentId={PARENT_ID} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ChildIncidentsWidget", () => {
  it("lists each child with its priority, state and assignee", async () => {
    postMock.mockResolvedValue({
      incidents: [
        {
          id: "c1",
          number: "INC0020001",
          subject: "Login fails for one tenant",
          priority: "HIGH",
          state: "IN_PROGRESS",
          assignedTo: { id: "u1", name: "Jane Doe" },
        },
        { id: "c2", number: "INC0020002", subject: "Same, another tenant", priority: "LOW", state: "RESOLVED" },
      ],
      total: 2,
      limit: 50,
      offset: 0,
    });
    renderWidget();

    expect(await screen.findByText("Child incidents (2)")).toBeInTheDocument();
    expect(screen.getByText("INC0020001 — Login fails for one tenant")).toBeInTheDocument();
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("Resolved")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("opens a child's page, with Back returning to this incident's Related tab", async () => {
    postMock.mockResolvedValue({
      incidents: [{ id: "c1", number: "INC0020001", subject: "Login fails", state: "NEW" }],
      total: 1,
      limit: 50,
      offset: 0,
    });
    renderWidget();

    const link = await screen.findByRole("link", { name: "INC0020001 — Login fails" });
    expect(link).toHaveAttribute("href", "/operations/incidents/c1");

    fireEvent.click(screen.getByText("New"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/incidents/c1", {
      state: { from: `/operations/incidents/${PARENT_ID}?tab=related` },
    });
  });

  it("says so when the incident has no children", async () => {
    postMock.mockResolvedValue({ incidents: [], total: 0, limit: 50, offset: 0 });
    renderWidget();
    expect(await screen.findByText("No child incidents.")).toBeInTheDocument();
  });

  it("reports a failed load instead of an empty list", async () => {
    postMock.mockRejectedValue(new Error("boom"));
    renderWidget();
    expect(
      await screen.findByText("Could not load the child incidents of this incident."),
    ).toBeInTheDocument();
  });
});
