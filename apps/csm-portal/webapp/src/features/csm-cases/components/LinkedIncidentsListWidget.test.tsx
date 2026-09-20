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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as LinkedChangeRequestsWidget.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { LinkedIncidentsListWidget } from "@features/csm-cases/components/LinkedIncidentsListWidget";

function renderWithProviders(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("LinkedIncidentsListWidget", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("searches /incidents/search filtered by this case's id as parentIds", async () => {
    postMock.mockResolvedValue({ incidents: [], total: 0, limit: 20, offset: 0 });

    renderWithProviders(<LinkedIncidentsListWidget caseId="case-1" />);

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/incidents/search", {
        filters: { parentIds: ["case-1"] },
        pagination: { offset: 0, limit: 20 },
      }),
    );
  });

  it("renders an empty state when there are no linked incidents", async () => {
    postMock.mockResolvedValue({ incidents: [], total: 0, limit: 20, offset: 0 });

    renderWithProviders(<LinkedIncidentsListWidget caseId="case-1" />);

    await waitFor(() =>
      expect(
        screen.getByText("No incidents linked to this case."),
      ).toBeInTheDocument(),
    );
  });

  it("renders each incident that has this case as its SN-side parent", async () => {
    postMock.mockResolvedValue({
      incidents: [
        {
          id: "inc-1",
          number: "INC0069722",
          subject: "Example incident one",
          priority: "HIGH",
          state: "IN_PROGRESS",
        },
        {
          id: "inc-2",
          number: "INC0074197",
          subject: "Example incident two",
          priority: "MODERATE",
          state: "NEW",
        },
      ],
      total: 2,
      limit: 20,
      offset: 0,
    });

    renderWithProviders(<LinkedIncidentsListWidget caseId="case-1" />);

    await waitFor(() =>
      expect(
        screen.getByText("INC0069722 — Example incident one"),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("INC0074197 — Example incident two")).toBeInTheDocument();
    expect(screen.getByText("Linked incidents (2)")).toBeInTheDocument();
    expect(screen.getByText("High")).toBeInTheDocument();
    expect(screen.getByText("Moderate")).toBeInTheDocument();
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("New")).toBeInTheDocument();
  });

  it("navigates to the incident's own detail route when a row is clicked", async () => {
    postMock.mockResolvedValue({
      incidents: [
        {
          id: "inc-1",
          number: "INC0069722",
          subject: "Example incident",
          priority: "HIGH",
          state: "IN_PROGRESS",
        },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    });

    renderWithProviders(<LinkedIncidentsListWidget caseId="case-1" />);

    const link = await screen.findByRole("link", {
      name: "INC0069722 — Example incident",
    });
    expect(link).toHaveAttribute("href", "/operations/incidents/inc-1");
    fireEvent.click(link);
  });

  it("renders an error state when the search fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    renderWithProviders(<LinkedIncidentsListWidget caseId="case-1" />);

    await waitFor(() =>
      expect(
        screen.getByText("Could not load linked incidents for this case."),
      ).toBeInTheDocument(),
    );
  });
});
