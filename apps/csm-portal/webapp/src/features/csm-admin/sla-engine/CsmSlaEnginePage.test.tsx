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
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CsmSlaEnginePage from "@features/csm-admin/sla-engine/CsmSlaEnginePage";

const api = vi.hoisted(() => ({
  statuses: [] as Array<Record<string, unknown>>,
  // Mirrors entity-service's real limit/offset paging against `statuses`, so
  // a test can simulate more than one page by seeding more rows than one
  // page holds, instead of every test implicitly only ever seeing page 0.
  get: vi.fn(async (url: string) => {
    const params = new URL(url, "http://example.test").searchParams;
    const limit = Number(params.get("limit") ?? api.statuses.length);
    const offset = Number(params.get("offset") ?? 0);
    return { statuses: api.statuses.slice(offset, offset + limit), total: api.statuses.length };
  }),
  post: vi.fn(async () => null),
}));

function makeStatus(caseId: string): Record<string, unknown> {
  return {
    caseId,
    clockType: "RESOLUTION",
    businessElapsedPercent: 10,
    hasBreached: false,
    isPaused: false,
    startedOn: null,
    pausedOn: null,
  };
}

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({
    get: api.get,
    post: api.post,
  }),
}));

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CsmSlaEnginePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  api.statuses = [];
  api.get.mockClear();
  api.post.mockClear();
});

describe("CsmSlaEnginePage", () => {
  it("shows an empty state when there are no active clocks", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText(/no active sla clocks/i)).toBeInTheDocument());
    expect(api.get).toHaveBeenCalledWith(expect.stringContaining("/sla-status/manage"));
  });

  it("renders active clocks returned by the backend", async () => {
    api.statuses = [
      {
        caseId: "11111111-1111-4111-8111-111111111111",
        clockType: "RESOLUTION",
        businessElapsedPercent: 42.5,
        hasBreached: false,
        isPaused: false,
        startedOn: "2026-01-05T10:00:00.000Z",
        pausedOn: null,
      },
    ];
    renderPage();
    await waitFor(() => expect(screen.getByText("11111111-1111-4111-8111-111111111111")).toBeInTheDocument());
    expect(screen.getByText("RESOLUTION")).toBeInTheDocument();
    expect(screen.getByText("43%")).toBeInTheDocument();
  });

  it("asks for confirmation before recalculating, then publishes the request", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText(/no active sla clocks/i)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Recalculate SLA" }));
    expect(screen.getByText("Recalculate SLA?")).toBeInTheDocument();
    expect(api.post).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Recalculate" }));

    await waitFor(() => expect(api.post).toHaveBeenCalledWith("/sla-engine/recalculate", {}));
    await waitFor(() => expect(screen.getByText(/recalculation requested/i)).toBeInTheDocument());
  });

  it("cancelling the confirmation does not call the backend", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Recalculate SLA" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByText("Recalculate SLA?")).not.toBeInTheDocument());
    expect(api.post).not.toHaveBeenCalled();
  });

  it("shows an error when the recalculation request fails", async () => {
    api.post.mockRejectedValueOnce(new Error("Failed to start SLA recalculation."));
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Recalculate SLA" }));
    fireEvent.click(screen.getByRole("button", { name: "Recalculate" }));

    await waitFor(() => expect(screen.getByText("Failed to start SLA recalculation.")).toBeInTheDocument());
  });

  it("resets to the last valid page when the total shrinks between fetches", async () => {
    api.statuses = Array.from({ length: 60 }, (_, i) => makeStatus(`case-${i}`));
    renderPage();
    await waitFor(() => expect(screen.getByText(/currently active clocks \(60\)/i)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Go to next page" }));
    await waitFor(() => expect(screen.getByText("case-25")).toBeInTheDocument());

    // The set shrinks to 10 rows (a recalculation, clocks completing) --
    // page 1 (offset 25) is now past the last page.
    api.statuses = Array.from({ length: 10 }, (_, i) => makeStatus(`case-${i}`));
    fireEvent.click(screen.getByRole("button", { name: "Recalculate SLA" }));
    fireEvent.click(screen.getByRole("button", { name: "Recalculate" }));

    await waitFor(() => expect(screen.getByText("case-0")).toBeInTheDocument());
    expect(screen.queryByText(/no active sla clocks right now/i)).not.toBeInTheDocument();
  });
});
